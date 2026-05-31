package jwt

import (
	"time"

	"github.com/world-future/fatecast-be/utils"
	"github.com/dgrijalva/jwt-go"
)

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func GenerateToken(secret, username string, duration time.Duration) (string, error) {
	claims := Claims{
		username,
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(duration).Unix(), // 过期时间
		},
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tokenClaims.SignedString(utils.StringToBytes(secret))
}

func ParseToken(secret, token string) (*Claims, error) {
	tokenClaims, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (any, error) {
		return utils.StringToBytes(secret), nil
	})

	if tokenClaims != nil {
		if claims, ok := tokenClaims.Claims.(*Claims); ok && tokenClaims.Valid {
			return claims, nil
		}
	}

	return nil, err
}
