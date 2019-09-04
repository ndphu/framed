package main

import (
	"frame-daemon/camera"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"log"
	"time"
)

func main() {
	cam := camera.NewCamera("/dev/video0", 640, 480)
	if err := cam.Start(); err != nil {
		panic(err)
	}

	defer cam.Stop()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type", "Content-Length", "X-Requested-With", "Connection", "Upgrade"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/api/capture", func(c *gin.Context) {
		frame := cam.GetFrame()
		c.Writer.Header().Set("Content-Type", "image/jpeg")
		c.Writer.Write(frame)
	})
	r.Run()

	log.Println("done")
}
