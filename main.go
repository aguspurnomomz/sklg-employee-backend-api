package main

import (
	"context"
	"net/http"
	"os"
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

// Struktur input untuk Tambah/Ubah Pegawai
type EmployeeRequest struct {
	FullName     string `json:"full_name" binding:"required"`
	PositionType string `json:"position_type" binding:"required"`
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

	// Endpoint Login
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
	
	// Protected Routes dengan Multi-Tenancy & RBAC Middleware
	api := r.Group("/api/v1")
	api.Use(middleware.MultiTenancyAuthMiddleware())
	{
		// 1. READ: Mengambil semua data pegawai berdasarkan school_id (Admin & Teacher)
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

		// 2. CREATE: Menambah data pegawai baru (Khusus School Admin)[cite: 4]
		api.POST("/employees", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)

			var req EmployeeRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid: full_name dan position_type wajib diisi"})
				return
			}

			query := `INSERT INTO employees (school_id, full_name, position_type) VALUES ($1, $2, $3) RETURNING id`
			var newID string
			err := database.DB.QueryRow(context.Background(), query, schoolID, req.FullName, req.PositionType).Scan(&newID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data pegawai ke database"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"message":     "Pegawai berhasil ditambahkan",
				"employee_id": newID,
				"school_id":   schoolID,
			})
		})

		// 3. UPDATE: Mengubah data pegawai berdasarkan ID (Khusus School Admin)[cite: 4]
		api.PUT("/employees/:id", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			employeeID := c.Param("id")

			var req EmployeeRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Format input tidak valid"})
				return
			}

			query := `UPDATE employees SET full_name = $1, position_type = $2 WHERE id = $3 AND school_id = $4`
			commandTag, err := database.DB.Exec(context.Background(), query, req.FullName, req.PositionType, employeeID, schoolID)
			if err != nil || commandTag.RowsAffected() == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pegawai tidak ditemukan atau akses ditolak"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":     "Data pegawai berhasil diperbarui",
				"employee_id": employeeID,
			})
		})

		// 4. DELETE: Menghapus data pegawai berdasarkan ID (Khusus School Admin)[cite: 4]
		api.DELETE("/employees/:id", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			employeeID := c.Param("id")

			query := `DELETE FROM employees WHERE id = $1 AND school_id = $2`
			commandTag, err := database.DB.Exec(context.Background(), query, employeeID, schoolID)
			if err != nil || commandTag.RowsAffected() == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pegawai tidak ditemukan atau akses ditolak"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":     "Data pegawai berhasil dihapus",
				"employee_id": employeeID,
			})
		})
	}

	// Menjalankan server dengan port dinamis untuk Cloud Run / lokal
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}