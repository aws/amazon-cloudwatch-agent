### OpenTelemetry JMX Metric Gatherer JAR

The OpenTelemetry JMX Metric Gatherer JAR ([code](https://github.com/open-telemetry/opentelemetry-java-contrib/tree/main/jmx-metrics)) is used by the [jmxreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/jmxreceiver#jar_path-default-optopentelemetry-java-contrib-jmx-metricsjar) to collect JMX metrics.
The JAR can be downloaded from the [release page](https://github.com/open-telemetry/opentelemetry-java-contrib/releases) of the OpenTelemetry Java Contrib. To update the version:
1. Run the `update-jmx-jar.sh` script to fetch and replace the JAR.
2. Update the SHA and version expected in the unit test (`opentelemetry_jmx_jar_test.go`).
3. Verify that the `jmxreceiver` used by the agent also has that version in `supported_jars.go`.
4. Run the integration tests against the new version.