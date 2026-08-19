package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var JwtSecret = []byte("KUNCI_RAHASIA_SUPER_AMAN_ANDA_123") 

type AppClaims struct {
	UserID   string `json:"user_id"`
	SchoolID string `json:"school_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}


func MultiTenancyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token otorisasi tidak ditemukan"})
			c.Abort()
			return
		}

		tokenString := strings.Split(authHeader, " ")
		if len(tokenString) != 2 || tokenString[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Format token salah"})
			c.Abort()
			return
		}

		claims := &AppClaims{}
		token, err := jwt.ParseWithClaims(tokenString[1], claims, func(token *jwt.Token) (interface{}, error) {
			return JwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak valid atau sudah kedaluwarsa"})
			c.Abort()
			return
		}

	
		c.Set("user_id", claims.UserID)
		c.Set("school_id", claims.SchoolID)
		c.Set("role", claims.Role)

		c.Next()
	}
}


func GetSchoolID(c *gin.Context) string {
	if val, exists := c.Get("school_id"); exists {
		return val.(string)
	}
	return ""
}


func GetRole(c *gin.Context) string {
	if val, exists := c.Get("role"); exists {
		return val.(string)
	}
	return ""
}


func RequireRoles(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetRole(c)

	
		isAllowed := false
		for _, role := range allowedRoles {
			if userRole == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Akses ditolak: Anda tidak memiliki hak akses (role)",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}