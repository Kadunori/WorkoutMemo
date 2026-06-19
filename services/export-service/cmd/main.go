// export-service は workout-service と user-service を呼び出してCSVを生成・返却する。
// K8s では Deployment (HTTPエンドポイント) と CronJob (定期S3アップロード) の両形式で使用できる。
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type WorkoutSet struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	ExerciseName string    `json:"exercise_name"`
	Equipment    string    `json:"equipment"`
	SetNumber    int       `json:"set_number"`
	Weight       float64   `json:"weight"`
	Reps         int       `json:"reps"`
	RIR          *int      `json:"rir"`
	CreatedAt    time.Time `json:"created_at"`
}

type BodyWeightRecord struct {
	ID         string    `json:"id"`
	Weight     float64   `json:"weight"`
	RecordedAt time.Time `json:"recorded_at"`
}

func jwtMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected method")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", claims["sub"].(string))
		c.Set("raw_token", tokenStr)
		c.Next()
	}
}

func fetchJSON(url, token string, out any) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return json.Unmarshal(body, out)
}

func exportWorkouts(c *gin.Context) {
	token := c.GetString("raw_token")
	workoutURL := os.Getenv("WORKOUT_SERVICE_URL") // e.g. http://workout-service:8080

	var sets []WorkoutSet
	if err := fetchJSON(workoutURL+"/workouts/last-set?export=all", token, &sets); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch workouts: " + err.Error()})
		return
	}

	var weights []BodyWeightRecord
	userURL := os.Getenv("USER_SERVICE_URL")
	_ = fetchJSON(userURL+"/users/weight", token, &weights)

	c.Header("Content-Disposition", "attachment; filename=workout_export.csv")
	c.Header("Content-Type", "text/csv; charset=utf-8")

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"type", "date", "exercise", "equipment", "set", "weight(kg)", "reps", "rir"})
	for _, s := range sets {
		rir := ""
		if s.RIR != nil {
			rir = fmt.Sprintf("%d", *s.RIR)
		}
		_ = w.Write([]string{
			"workout",
			s.CreatedAt.Format("2006-01-02"),
			s.ExerciseName,
			s.Equipment,
			fmt.Sprintf("%d", s.SetNumber),
			fmt.Sprintf("%.2f", s.Weight),
			fmt.Sprintf("%d", s.Reps),
			rir,
		})
	}
	for _, bw := range weights {
		_ = w.Write([]string{
			"bodyweight",
			bw.RecordedAt.Format("2006-01-02"),
			"", "", "",
			fmt.Sprintf("%.1f", bw.Weight),
			"", "",
		})
	}
	w.Flush()
}

func main() {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "export"}) })

	auth := r.Group("/", jwtMiddleware(os.Getenv("JWT_SECRET")))
	auth.GET("/export/workouts", exportWorkouts)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("export-service :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
