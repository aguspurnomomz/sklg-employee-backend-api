package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var JwtSecret = []byte("KUNCI_RAHASIA_SUPER_AMAN_ANDA_123") // Sesuaikan dengan secret yang Anda gunakan

type AppClaims struct {
	UserID   string `json:"user_id"`
	SchoolID string `json:"school_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Middleware Multi-Tenancy & Auth Dasar
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

		// Simpan data klaim ke context Gin agar bisa dipakai handler selanjutnya
		c.Set("user_id", claims.UserID)
		c.Set("school_id", claims.SchoolID)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// Helper untuk mengambil School ID
func GetSchoolID(c *gin.Context) string {
	if val, exists := c.Get("school_id"); exists {
		return val.(string)
	}
	return ""
}

// Helper untuk mengambil Role
func GetRole(c *gin.Context) string {
	if val, exists := c.Get("role"); exists {
		return val.(string)
	}
	return ""
}

// --- BARU: Middleware RBAC (Role-Based Access Control) ---
func RequireRoles(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetRole(c)

		// Cek apakah role user ada di dalam daftar role yang diizinkan
		isAllowed := false
		for _, role := range allowedRoles {
			if userRole == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Akses ditolak: Anda tidak memiliki hak akses (role) yang diperlukan",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}