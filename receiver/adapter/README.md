# Adapter Receiver

The Adapter Receiver receives Telegraf metrics, filters unsupported value
and converts them to corresponding OTEL metrics before passing down to
OTEL processors and exporters. This is intended to be used when Telegraf 
input plugins are still intact.

Supported pipeline types: metrics

