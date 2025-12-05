package auth

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

/*
 * Example of JWT token with user custom claims:
	{
		"iss": "Authz server",
		"exp": 1762981529,
		"iat": 1762959929,
		"custom_claims": {
			"user": "username",
			"collection": "mycollection"
		}
	}
*/

// CustomClaims defines custom claims of JWT token
type CustomClaims struct {
	User       string `json:"user"`
	Collection string `json:"collection"`
}

// Claims defines JWT claims to parse
type Claims struct {
	Custom CustomClaims `json:"custom_claims"`
	jwt.RegisteredClaims
}

// ParseToken provides JWT token parser
func ParseToken(tokenString string, jwtSecret []byte) (*User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	return &User{
		Name:       claims.Custom.User,
		Collection: claims.Custom.Collection,
	}, nil
}
