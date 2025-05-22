import os
import logging
import grpc
import signal
from concurrent import futures

from azure.monitor.opentelemetry.exporter import AzureMonitorTraceExporter
from opentelemetry import trace
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.trace import SpanKind
from opentelemetry.trace import SpanContext, TraceFlags, TraceState
from opentelemetry.proto.collector.trace.v1 import (
    trace_service_pb2,
    trace_service_pb2_grpc,
)
from opentelemetry.proto.trace.v1.trace_pb2 import Span as ProtoSpan

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("image-customizer-telemetry")

AZURE_CONN_STR = "InstrumentationKey=c0b360fa-422d-40e5-b8a9-d642578f9fce;IngestionEndpoint=https://eastus-8.in.applicationinsights.azure.com/;LiveEndpoint=https://eastus.livediagnostics.monitor.azure.com/;ApplicationId=087d527c-b60e-4346-a679-f6abf367d0f0"

provider = TracerProvider(resource=Resource.create({}))
trace.set_tracer_provider(provider)

# Instantiate the Azure Monitor exporter once
exporter = AzureMonitorTraceExporter(connection_string=AZURE_CONN_STR)


class SpanData:
    """
    SpanData class allows mapping incoming trace data to spans that get
    processed by AzureMonitorTraceExporter.
    """

    def __init__(self, proto_span, resource_attrs, inst_scope):
        # span name and timing
        self.name = proto_span.name
        self.start_time = proto_span.start_time_unix_nano
        self.end_time = proto_span.end_time_unix_nano

        # map OTLP Span.Kind → Python SpanKind
        proto_kind = proto_span.kind
        if proto_kind == ProtoSpan.SPAN_KIND_SERVER:
            self.kind = SpanKind.SERVER
        elif proto_kind == ProtoSpan.SPAN_KIND_CLIENT:
            self.kind = SpanKind.CLIENT
        elif proto_kind == ProtoSpan.SPAN_KIND_PRODUCER:
            self.kind = SpanKind.PRODUCER
        elif proto_kind == ProtoSpan.SPAN_KIND_CONSUMER:
            self.kind = SpanKind.CONSUMER
        elif proto_kind == ProtoSpan.SPAN_KIND_INTERNAL:
            self.kind = SpanKind.INTERNAL
        else:
            self.kind = SpanKind.INTERNAL

        # merge resource + span attributes (string only)
        attrs = resource_attrs.copy()
        for kv in proto_span.attributes:
            if kv.value.WhichOneof("value") == "string_value":
                attrs[kv.key] = kv.value.string_value
        self.attributes = attrs

        # status code if present
        self.status = None
        # Map proto status → SDK Status object
        if proto_span.HasField("status"):
            code = StatusCode(proto_span.status.code)
            desc = proto_span.status.message or None
            self.status = Status(status_code=code, description=desc)
        else:
            # Use UNSET so is_ok returns True
            self.status = Status(status_code=StatusCode.UNSET)

        # events and links (empty lists here)
        self.events = []
        self.links = []

        # reconstruct trace & parent contexts
        tid = int.from_bytes(proto_span.trace_id, "big")
        sid = int.from_bytes(proto_span.span_id, "big")
        self.context = SpanContext(
            trace_id=tid,
            span_id=sid,
            is_remote=True,
            trace_flags=TraceFlags(0),
            trace_state=TraceState(),
        )
        if proto_span.parent_span_id:
            psid = int.from_bytes(proto_span.parent_span_id, "big")
            self.parent = SpanContext(
                trace_id=tid,
                span_id=psid,
                is_remote=True,
                trace_flags=TraceFlags(0),
                trace_state=TraceState(),
            )
        else:
            self.parent = None

        # resource and instrumentation scope/library
        self.resource = Resource.create(resource_attrs)
        self.instrumentation_scope = inst_scope
        self.instrumentation_library = inst_scope


class TraceServiceHandler(trace_service_pb2_grpc.TraceServiceServicer):
    def Export(self, request, context):
        logger.info("Received %d Spans", len(request.resource_spans))

        spans = []
        for rs in request.resource_spans:
            # extract resource-level attributes
            resource_attrs = {
                kv.key: kv.value.string_value
                for kv in rs.resource.attributes
                if kv.value.WhichOneof("value") == "string_value"
            }

            for ss in rs.scope_spans:
                # OTLP stub may call this field instrumentation_scope or
                # instrumentation_library
                inst_scope = getattr(
                    ss,
                    "instrumentation_scope",
                    getattr(ss, "instrumentation_library", None),
                )
                for proto_span in ss.spans:
                    spans.append(SpanData(proto_span, resource_attrs, inst_scope))

        # forward the entire batch to Azure Monitor
        exporter.export(spans)

        return trace_service_pb2.ExportTraceServiceResponse()


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    trace_service_pb2_grpc.add_TraceServiceServicer_to_server(
        TraceServiceHandler(), server
    )
    server.add_insecure_port("[::]:4317")
    server.start()
    logger.info("Listening on port 4317 for OTLP traces")

    def shutdown_handler(*_):
        logger.info("Shutdown received, stopping server")
        server.stop(0)

    signal.signal(signal.SIGINT, shutdown_handler)
    signal.signal(signal.SIGTERM, shutdown_handler)

    server.wait_for_termination()


if __name__ == "__main__":
    serve()
