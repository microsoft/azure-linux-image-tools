import os
import logging
import grpc
import signal
from concurrent import futures
from typing import Dict, Any, List, Optional

from azure.monitor.opentelemetry.exporter import AzureMonitorTraceExporter
from opentelemetry import trace
from opentelemetry.trace.status import Status, StatusCode
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.trace import SpanContext, TraceFlags, TraceState
from opentelemetry.proto.collector.trace.v1 import (
    trace_service_pb2,
    trace_service_pb2_grpc,
)
from opentelemetry.proto.trace.v1.trace_pb2 import Span as ProtoSpan
from opentelemetry.proto.common.v1.common_pb2 import KeyValue

DEFAULT_GRPC_PORT = 4317
MAX_WORKERS = 10

AZURE_CONN_STR = "InstrumentationKey=c0b360fa-422d-40e5-b8a9-d642578f9fce;IngestionEndpoint=https://eastus-8.in.applicationinsights.azure.com/;LiveEndpoint=https://eastus.livediagnostics.monitor.azure.com/;ApplicationId=087d527c-b60e-4346-a679-f6abf367d0f0"


logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("image-customizer-telemetry")


class SpanData:
    """SpanData class for Azure Monitor export."""

    def __init__(
        self, proto_span: ProtoSpan, resource_attrs: Dict[str, Any], inst_scope: Any
    ) -> None:
        """Initialize SpanData from protocol buffer span."""
        try:
            self.name = proto_span.name
            self.start_time = proto_span.start_time_unix_nano
            self.end_time = proto_span.end_time_unix_nano
            self.kind = proto_span.kind

            self.attributes = self._merge_attributes(
                proto_span.attributes, resource_attrs
            )
            self.status = self._extract_status(proto_span)
            self.events = proto_span.events
            self.links = proto_span.links
            self.context = self._create_span_context(proto_span)
            self.parent = self._create_parent_context(proto_span)
            self.resource = Resource.create(resource_attrs)
            self.instrumentation_scope = inst_scope

        except Exception as e:
            logger.error(f"Failed to initialize SpanData: {e}")
            raise

    def _merge_attributes(
        self, proto_attributes: List[KeyValue], resource_attrs: Dict[str, Any]
    ) -> Dict[str, Any]:
        """Merge resource and span attributes."""
        attributes = dict(resource_attrs)

        span_attrs = extract_attributes_from_proto(proto_attributes)
        attributes.update(span_attrs)

        return attributes

    def _extract_status(self, proto_span: ProtoSpan) -> Status:
        """Extract status from protocol buffer span."""
        if proto_span.HasField("status"):
            return Status(
                status_code=StatusCode(proto_span.status.code),
                description=proto_span.status.message or None,
            )
        return Status(StatusCode.UNSET)

    def _create_span_context(self, proto_span: ProtoSpan) -> SpanContext:
        """Create SpanContext from protocol buffer span."""
        return SpanContext(
            trace_id=int.from_bytes(proto_span.trace_id, "big"),
            span_id=int.from_bytes(proto_span.span_id, "big"),
            is_remote=True,
            trace_flags=TraceFlags(0),
            trace_state=TraceState(),
        )

    def _create_parent_context(self, proto_span: ProtoSpan) -> Optional[SpanContext]:
        """Create parent SpanContext if parent span ID exists."""
        if not proto_span.parent_span_id:
            return None

        return SpanContext(
            trace_id=int.from_bytes(proto_span.trace_id, "big"),
            span_id=int.from_bytes(proto_span.parent_span_id, "big"),
            is_remote=True,
            trace_flags=TraceFlags(0),
            trace_state=TraceState(),
        )


def initialize_telemetry() -> AzureMonitorTraceExporter:
    """Initialize OpenTelemetry and Azure Monitor exporter."""
    provider = TracerProvider(resource=Resource.create({}))
    trace.set_tracer_provider(provider)

    return AzureMonitorTraceExporter(connection_string=AZURE_CONN_STR)


class TraceServiceHandler(trace_service_pb2_grpc.TraceServiceServicer):
    """OTLP trace service handler that forwards traces to Azure Monitor."""

    def __init__(self) -> None:
        """Initialize the trace service handler."""
        self.exporter = initialize_telemetry()

    def Export(self, request, context) -> trace_service_pb2.ExportTraceServiceResponse:
        """Export traces to Azure Monitor."""
        try:
            spans = self._process_trace_request(request)

            if spans:
                result = self.exporter.export(spans)
                logger.info(
                    "Successfully exported %d spans to Azure Monitor (result: %s)",
                    len(spans),
                    result,
                )
            return trace_service_pb2.ExportTraceServiceResponse()

        except Exception as e:
            logger.error("Error processing spans: %s", e, exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Failed to process spans: {str(e)}")
            return trace_service_pb2.ExportTraceServiceResponse()

    def _process_trace_request(self, request) -> List[SpanData]:
        """Process trace request and convert to SpanData objects."""
        spans = []

        for rs in request.resource_spans:
            resource_attrs = extract_attributes_from_proto(rs.resource.attributes)

            for ss in rs.scope_spans:
                for proto_span in ss.spans:
                    try:
                        span_data = SpanData(proto_span, resource_attrs, ss.scope)
                        spans.append(span_data)
                    except Exception as e:
                        logger.warning(f"Failed to process span {proto_span.name}: {e}")

        return spans


# Utility functions for protobuf attribute extraction
def extract_attribute_value(value_proto: Any) -> Optional[Any]:
    """Extract value from protobuf AnyValue."""
    value_case = value_proto.WhichOneof("value")
    value_mapping = {
        "string_value": value_proto.string_value,
        "int_value": value_proto.int_value,
        "double_value": value_proto.double_value,
        "bool_value": value_proto.bool_value,
    }
    return value_mapping.get(value_case)


def extract_attributes_from_proto(proto_attributes: List[KeyValue]) -> Dict[str, Any]:
    """Extract attributes from protobuf KeyValue pairs."""
    attributes = {}
    for kv in proto_attributes:
        value = extract_attribute_value(kv.value)
        if value is not None:
            attributes[kv.key] = value
    return attributes


def create_server() -> grpc.Server:
    """Create and configure the gRPC server."""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=MAX_WORKERS))
    trace_service_pb2_grpc.add_TraceServiceServicer_to_server(
        TraceServiceHandler(), server
    )
    server.add_insecure_port(f"[::]:{DEFAULT_GRPC_PORT}")
    return server


def setup_signal_handlers(server: grpc.Server) -> None:
    """Setup signal handlers for graceful shutdown."""

    def shutdown_handler(signum, frame):
        logger.info(f"Received signal {signum}, stopping server gracefully")
        server.stop(grace=5)

    signal.signal(signal.SIGINT, shutdown_handler)
    signal.signal(signal.SIGTERM, shutdown_handler)


def serve() -> None:
    """Start the telemetry forwarding server."""
    try:
        server = create_server()
        setup_signal_handlers(server)

        server.start()
        logger.info(
            f"Telemetry hopper listening on port {DEFAULT_GRPC_PORT} for OTLP traces"
        )

        server.wait_for_termination()
        logger.info("Server stopped")

    except Exception as e:
        logger.error(f"Failed to start server: {e}")
        raise


if __name__ == "__main__":
    serve()
