# syntax=docker/dockerfile:1

FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY pom.xml .
RUN mvn -q -DskipTests dependency:go-offline
COPY src ./src
RUN mvn -q -DskipTests package

FROM eclipse-temurin:21-jre AS api
WORKDIR /app
COPY --from=build /src/target/payment-service-0.1.0.jar /app/app.jar
# OpenTelemetry Java agent: auto-instruments Spring MVC (extract incoming trace
# context) and the Kafka producer (inject it into payments.events). Baked into
# the image; activated and configured at runtime via env in the deployment.
ADD https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.9.0/opentelemetry-javaagent.jar /app/opentelemetry-javaagent.jar
EXPOSE 8080
USER 1000:1000
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
