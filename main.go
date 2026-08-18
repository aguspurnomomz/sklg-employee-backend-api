package main

import (
	"context"
	"net/http"
	"os" // <-- 1. Tambahkan import "os" di sini
	"time"

	"my-school-saas/database"
	"my-school-saas/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func generateJWT(userID, schoolID, role string) (string, error) {
	claims := middleware.AppClaims{
		UserID:   userID,
		SchoolID: schoolID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(middleware.JwtSecret)
}

func main() {
	database.ConnectDB()

	r := gin.Default()

	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid"})
			return
		}

		var userID, schoolID, passwordHash string
		role := "SCHOOL_ADMIN" 
		
		query := `
			SELECT u.id, usr.school_id, u.password_hash 
			FROM users u
			JOIN user_school_roles usr ON u.id = usr.user_id
			WHERE u.email = $1
		`

		err := database.DB.QueryRow(context.Background(), query, req.Email).Scan(&userID, &schoolID, &passwordHash)
		if err != nil {
			println("DB Error:", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau password salah"})
			return
		}

		if passwordHash != req.Password {
			if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau password salah"})
				return
			}
		}

		token, err := generateJWT(userID, schoolID, role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat token akses"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Login berhasil",
			"token":     token,
			"user_id":   userID,
			"school_id": schoolID,
			"role":      role,
		})
	})
	
	api := r.Group("/api/v1")
	api.Use(middleware.MultiTenancyAuthMiddleware())
	{
		api.GET("/employees", middleware.RequireRoles("SCHOOL_ADMIN", "TEACHER"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			role := middleware.GetRole(c)

			rows, err := database.DB.Query(context.Background(), "SELECT id, full_name, position_type FROM employees WHERE school_id = $1", schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pegawai"})
				return
			}
			defer rows.Close()

			type Employee struct {
				ID           string `json:"id"`
				FullName     string `json:"full_name"`
				PositionType string `json:"position_type"`
			}

			var employees []Employee
			for rows.Next() {
				var emp Employee
				if err := rows.Scan(&emp.ID, &emp.FullName, &emp.PositionType); err == nil {
					employees = append(employees, emp)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"message":   "Berhasil mengambil data pegawai",
				"school_id": schoolID,
				"role":      role,
				"data":      employees,
			})
		})

		api.POST("/employees", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			
			c.JSON(http.StatusOK, gin.H{
				"message":   "Akses diizinkan: Fitur tambah pegawai khusus Admin Sekolah",
				"school_id": schoolID,
			})
		})
	}

	// 2. Ambil port dari environment variable Cloud Run, gunakan "8080" jika dijalankan lokal
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}