package com.orderfulfillment.payment.config;

import java.util.Arrays;
import java.util.List;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.cors.CorsConfiguration;
import org.springframework.web.cors.UrlBasedCorsConfigurationSource;
import org.springframework.web.filter.CorsFilter;

/**
 * Allows the separately deployed web console to call the payment API from
 * another origin. Origins come from cors.allowed-origins (CORS_ALLOWED_ORIGINS);
 * "*" allows any. Empty disables CORS.
 */
@Configuration
public class WebCorsConfig {

    @Bean
    public CorsFilter corsFilter(@Value("${cors.allowed-origins:}") String allowedOrigins) {
        List<String> origins = Arrays.stream(allowedOrigins.split(","))
                .map(String::trim)
                .filter(s -> !s.isEmpty())
                .toList();

        CorsConfiguration config = new CorsConfiguration();
        if (origins.contains("*")) {
            config.addAllowedOriginPattern("*");
        } else {
            origins.forEach(config::addAllowedOrigin);
        }
        config.setAllowedMethods(List.of("GET", "POST", "PUT", "OPTIONS"));
        config.setAllowedHeaders(List.of("Content-Type", "Idempotency-Key", "Authorization"));
        config.setMaxAge(600L);

        UrlBasedCorsConfigurationSource source = new UrlBasedCorsConfigurationSource();
        source.registerCorsConfiguration("/**", config);
        return new CorsFilter(source);
    }
}
