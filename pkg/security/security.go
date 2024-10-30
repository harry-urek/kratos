package security

type SecurityMiddleware struct {
	rateLimiter    *Limiter
	hmacKey        []byte
	allowedOrigins map[string]bool
}

func NewSecurity(hmacKey []byte, allowedOrigins []string) *SecurityMiddleware {
	origins := make(map[string]bool)
	for _, origin := range allowedOrigins {
		origins[origin] = true
	}

	return &SecurityMiddleware{
		rateLimiter:    NewRateLimiter(5, 10),
		hmacKey:        hmacKey,
		allowedOrigins: origins,
	}

}
