// Package fallback provides a multi-provider failover chain with error
// classification (auth/rate_limit/timeout/format/overloaded) and exponential
// backoff cooldown for failed providers.
package fallback
