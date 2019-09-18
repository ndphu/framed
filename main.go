package main

import (
	"encoding/base64"
	"frame-daemon/recognize"
	"github.com/Kagami/go-face"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"image"
	"log"
	"time"
)

var recWrapper = recognize.RecognizerWrapper{}

type ImageInput struct {
	Payload string `json:"payload"`
}

func (i *ImageInput) ToBytes() []byte {
	if data, err := base64.StdEncoding.DecodeString(i.Payload); err != nil {
		return nil
	} else {
		return data
	}
}

func main() {
	rec, err := face.NewRecognizer("data")
	if err != nil {
		log.Fatalf("fail to initialize recognizer. %v\n", err)
	}
	defer rec.Close()

	recWrapper.Recognizer = rec

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type", "Content-Length", "X-Requested-With", "Connection", "Upgrade"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}))

	r.POST("/api/detectFaces", func(c *gin.Context) {
		input := ImageInput{}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
		} else {
			if frame := input.ToBytes(); frame != nil {
				if faces, err := recWrapper.Recognizer.Recognize(frame); err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
				} else {
					imageB64 := base64.StdEncoding.EncodeToString(frame)

					var dfs []DetectedFace

					if faces != nil && len(faces) > 0 {
						for _, f := range faces {
							dfs = append(dfs, DetectedFace{
								Rect:       f.Rectangle,
								Descriptor: f.Descriptor,
							})
						}
					}

					c.JSON(200, gin.H{
						"image":         imageB64,
						"detectedFaces": dfs,
					})
				}
			} else {
				c.JSON(500, gin.H{"error": "empty image"})
			}
		}
	})

	r.POST("/api/recognizeFaces", func(c *gin.Context) {
		input := ImageInput{}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
		} else {
			if frame := input.ToBytes(); frame != nil {
				if faces, err := recWrapper.Recognizer.Recognize(frame); err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
				} else {
					imageB64 := base64.StdEncoding.EncodeToString(frame)
					var rfs []RecognizedFace
					if faces != nil {
						for _, f := range faces {
							classified := recWrapper.Recognizer.ClassifyThreshold(f.Descriptor, 0.15)
							if classified >= 0 {
								rfs = append(rfs, RecognizedFace{
									Rect:       f.Rectangle,
									Label:      recWrapper.Categories[int32(classified)],
									Classified: classified,
								})
							} else {
								rfs = append(rfs, RecognizedFace{
									Rect:       f.Rectangle,
									Label:      "UNKNOWN",
									Classified: classified,
								})
							}
						}
					}

					c.JSON(200, gin.H{
						"image":           imageB64,
						"recognizedFaces": rfs,
					})
				}
			} else {
				c.JSON(500, gin.H{"error": "empty image"})
			}
		}
	})

	r.POST("/api/reloadSamples", func(c *gin.Context) {
		if err := recognize.ReloadSamples(&recWrapper); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		} else {
			c.JSON(200, gin.H{"error": ""})
		}
	})

	r.Run()
}

type RecognizedFace struct {
	Rect       image.Rectangle `json:"rect"`
	Label      string          `json:"label"`
	Classified int             `json:"category"`
}

type DetectedFace struct {
	Rect       image.Rectangle `json:"rect"`
	Descriptor face.Descriptor `json:"descriptor"`
}
