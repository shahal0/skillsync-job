package middleware

// Claims represents the structure of JWT token claims
type Claims struct {
	UserID string
	Role   string
	// Add other fields as needed
}
