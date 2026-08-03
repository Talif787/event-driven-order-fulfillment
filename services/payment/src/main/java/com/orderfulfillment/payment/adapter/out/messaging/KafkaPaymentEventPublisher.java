package com.orderfulfillment.payment.adapter.out.messaging;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.orderfulfillment.payment.application.port.PaymentEventPublisher;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

import java.nio.charset.StandardCharsets;

/**
 * Publishes payment events to the backbone. The event type travels in the
 * "event-type" header (matching the Go services' convention) so consumers route
 * without decoding the body, and records are keyed by order id so a given
 * order's events keep their order on the partition.
 */
@Component
public class KafkaPaymentEventPublisher implements PaymentEventPublisher {

    private static final Logger log = LoggerFactory.getLogger(KafkaPaymentEventPublisher.class);
    private static final String HEADER_EVENT_TYPE = "event-type";

    private final KafkaTemplate<String, String> kafka;
    private final ObjectMapper mapper;
    private final String topic;

    public KafkaPaymentEventPublisher(KafkaTemplate<String, String> kafka,
                                      ObjectMapper mapper,
                                      @Value("${payment.events-topic}") String topic) {
        this.kafka = kafka;
        this.mapper = mapper;
        this.topic = topic;
    }

    @Override
    public void publish(PaymentEvent event) {
        String payload;
        try {
            payload = mapper.writeValueAsString(event);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("marshal payment event", e);
        }
        ProducerRecord<String, String> record =
                new ProducerRecord<>(topic, event.orderId().toString(), payload);
        record.headers().add(HEADER_EVENT_TYPE, event.type().getBytes(StandardCharsets.UTF_8));
        kafka.send(record);
        log.info("published {} orderId={}", event.type(), event.orderId());
    }
}
