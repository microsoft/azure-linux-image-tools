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
    Simplified SpanData class for Azure Monitor export.
    """

    def __init__(self, proto_span, resource_attrs, inst_scope):
        self.name = proto_span.name
        self.start_time = proto_span.start_time_unix_nano
        self.end_time = proto_span.end_time_unix_nano

        # Azure Monitor requires span kind - use INTERNAL as default
        self.kind = SpanKind.INTERNAL

        # Handle attributes - merge resource + span attributes
        self.attributes = {}

        # Add resource attributes first
        for key, value in resource_attrs.items():
            self.attributes[key] = value

        for kv in proto_span.attributes:
            value_case = kv.value.WhichOneof("value")
            if value_case == "string_value":
                self.attributes[kv.key] = kv.value.string_value
            elif value_case == "int_value":
                self.attributes[kv.key] = kv.value.int_value
            elif value_case == "double_value":
                self.attributes[kv.key] = kv.value.double_value
            elif value_case == "bool_value":
                self.attributes[kv.key] = kv.value.bool_value

        # Simple status handling
        if proto_span.HasField("status"):
            self.status = Status(
                status_code=StatusCode(proto_span.status.code),
                description=proto_span.status.message or None,
            )
        else:
            self.status = Status(status_code=StatusCode.UNSET)

        self.events = []
        self.links = []

        # Simple context creation
        self.context = SpanContext(
            trace_id=int.from_bytes(proto_span.trace_id, "big"),
            span_id=int.from_bytes(proto_span.span_id, "big"),
            is_remote=True,
            trace_flags=TraceFlags(0),
            trace_state=TraceState(),
        )

        self.parent = None
        if proto_span.parent_span_id:
            self.parent = SpanContext(
                trace_id=int.from_bytes(proto_span.trace_id, "big"),
                span_id=int.from_bytes(proto_span.parent_span_id, "big"),
                is_remote=True,
                trace_flags=TraceFlags(0),
                trace_state=TraceState(),
            )

        # Simplified resource
        self.resource = Resource.create(resource_attrs)
        self.instrumentation_scope = inst_scope


class TraceServiceHandler(trace_service_pb2_grpc.TraceServiceServicer):
    def Export(self, request, context):
        try:
            total_spans = sum(
                len(ss.spans) for rs in request.resource_spans for ss in rs.scope_spans
            )
            logger.info(
                "Received %d spans across %d resource spans",
                total_spans,
                len(request.resource_spans),
            )

            spans = []
            for rs in request.resource_spans:
                # Extract resource-level attributes (strings only)
                resource_attrs = {
                    kv.key: kv.value.string_value
                    for kv in rs.resource.attributes
                    if kv.value.WhichOneof("value") == "string_value"
                }

                for ss in rs.scope_spans:
                    # Get instrumentation scope
                    inst_scope = getattr(
                        ss,
                        "instrumentation_scope",
                        getattr(ss, "instrumentation_library", None),
                    )

                    # Convert all spans in this scope
                    for proto_span in ss.spans:
                        spans.append(SpanData(proto_span, resource_attrs, inst_scope))

            # Export all spans to Azure Monitor
            if spans:
                result = exporter.export(spans)
                logger.info(
                    "Exported %d spans to Azure Monitor (result: %s)",
                    len(spans),
                    result,
                )
            else:
                logger.warning("No spans to export")

            return trace_service_pb2.ExportTraceServiceResponse()

        except Exception as e:
            logger.error("Error processing spans: %s", e, exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Failed to process spans: {str(e)}")
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
