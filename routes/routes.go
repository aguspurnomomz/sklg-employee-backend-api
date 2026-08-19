package routes

import (
	"context"
	"net/http"
	"time"

	"my-school-saas/database"
	"my-school-saas/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type EmployeeRequest struct {
	EmployeeNumber string `json:"employee_number"`
	FullName       string `json:"full_name" binding:"required"`
	Gender         string `json:"gender"`
	PositionType   string `json:"position_type" binding:"required"`
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

func InitRoutes(r *gin.Engine) {
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

	api := r.Group("/api/v1")
	api.Use(middleware.MultiTenancyAuthMiddleware())
	{
	
		api.GET("/employees", middleware.RequireRoles("SCHOOL_ADMIN", "TEACHER"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			role := middleware.GetRole(c)

			var schoolName string
			_ = database.DB.QueryRow(context.Background(), "SELECT name FROM schools WHERE id = $1", schoolID).Scan(&schoolName)

			// 2. Query data pegawai
			rows, err := database.DB.Query(context.Background(), "SELECT id, employee_number, full_name, gender, position_type FROM employees WHERE school_id = $1", schoolID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  false,
					"message": "Gagal mengambil data pegawai",
				})
				return
			}
			defer rows.Close()

			type Employee struct {
				ID             string  `json:"id"`
				EmployeeNumber *string `json:"employee_number"`
				FullName       string  `json:"full_name"`
				Gender         *string `json:"gender"`
				PositionType   string  `json:"position_type"`
			}

			var employees []Employee
			for rows.Next() {
				var emp Employee
				if err := rows.Scan(&emp.ID, &emp.EmployeeNumber, &emp.FullName, &emp.Gender, &emp.PositionType); err == nil {
					employees = append(employees, emp)
				}
			}

			if employees == nil {
				employees = []Employee{}
			}

			c.JSON(http.StatusOK, gin.H{
				"status":           true,
				"message":          "Berhasil mengambil data pegawai",
				"total_employees":  len(employees),
				"school_id":        schoolID,
				"school_name":      schoolName,
				"role":             role,
				"data":             employees,
			})
		})

	
		api.POST("/save_employee", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)

			var req EmployeeRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  false,
					"message": "Format input tidak valid",
				})
				return
			}

			if req.EmployeeNumber != "" {
				var existingCount int
				checkQuery := `SELECT COUNT(*) FROM employees WHERE school_id = $1 AND employee_number = $2`
				err := database.DB.QueryRow(context.Background(), checkQuery, schoolID, req.EmployeeNumber).Scan(&existingCount)
				if err == nil && existingCount > 0 {
					c.JSON(http.StatusConflict, gin.H{
						"status":  false,
						"message": "Nomor pegawai tersebut sudah terdaftar di sekolah ini",
					})
					return
				}
			}

			newID := uuid.New().String()

			query := `INSERT INTO employees (id, school_id, employee_number, full_name, gender, position_type) VALUES ($1, $2, $3, $4, $5, $6)`
			_, err := database.DB.Exec(context.Background(), query, newID, schoolID, req.EmployeeNumber, req.FullName, req.Gender, req.PositionType)
			if err != nil {
				println("SQL Insert Error:", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  false,
					"message": "Gagal menyimpan data pegawai: " + err.Error(),
				})
				return
			}

			var schoolName string
			_ = database.DB.QueryRow(context.Background(), "SELECT name FROM schools WHERE id = $1", schoolID).Scan(&schoolName)

			c.JSON(http.StatusCreated, gin.H{
				"status":      true,
				"message":     "Pegawai berhasil ditambahkan",
				"employee_id": newID,
				"school_id":   schoolID,
				"school_name": schoolName,
			})
		})

		api.PUT("/update_employee/:id", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			employeeID := c.Param("id")

			var req EmployeeRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  false,
					"message": "Format input tidak valid",
				})
				return
			}

			if req.EmployeeNumber != "" {
				var existingCount int
				checkQuery := `SELECT COUNT(*) FROM employees WHERE school_id = $1 AND employee_number = $2 AND id != $3`
				err := database.DB.QueryRow(context.Background(), checkQuery, schoolID, req.EmployeeNumber, employeeID).Scan(&existingCount)
				if err == nil && existingCount > 0 {
					c.JSON(http.StatusConflict, gin.H{
						"status":  false,
						"message": "Nomor pegawai (employee_number) tersebut sudah digunakan oleh pegawai lain di sekolah ini",
					})
					return
				}
			}

			query := `UPDATE employees SET employee_number = $1, full_name = $2, gender = $3, position_type = $4 WHERE id = $5 AND school_id = $6`
			commandTag, err := database.DB.Exec(context.Background(), query, req.EmployeeNumber, req.FullName, req.Gender, req.PositionType, employeeID, schoolID)
			if err != nil || commandTag.RowsAffected() == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  false,
					"message": "Pegawai tidak ditemukan atau akses ditolak",
				})
				return
			}

			var schoolName string
			_ = database.DB.QueryRow(context.Background(), "SELECT name FROM schools WHERE id = $1", schoolID).Scan(&schoolName)

			c.JSON(http.StatusOK, gin.H{
				"status":      true,
				"message":     "Data pegawai berhasil diperbarui",
				"employee_id": employeeID,
				"school_id":   schoolID,
				"school_name": schoolName,
			})
		})

		api.DELETE("/delete_employee/:id", middleware.RequireRoles("SCHOOL_ADMIN"), func(c *gin.Context) {
			schoolID := middleware.GetSchoolID(c)
			employeeID := c.Param("id")

			query := `DELETE FROM employees WHERE id = $1 AND school_id = $2`
			commandTag, err := database.DB.Exec(context.Background(), query, employeeID, schoolID)
			if err != nil || commandTag.RowsAffected() == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  false,
					"message": "Pegawai tidak ditemukan atau akses ditolak",
				})
				return
			}

			var schoolName string
			_ = database.DB.QueryRow(context.Background(), "SELECT name FROM schools WHERE id = $1", schoolID).Scan(&schoolName)

			c.JSON(http.StatusOK, gin.H{
				"status":      true,
				"message":     "Data pegawai berhasil dihapus",
				"employee_id": employeeID,
				"school_id":   schoolID,
				"school_name": schoolName,
			})
		})
	}
}