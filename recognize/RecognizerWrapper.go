package recognize

import (
	"encoding/json"
	"github.com/ndphu/go-face"
	"io/ioutil"
	"net/http"
)

type RecognizerWrapper struct {
	Recognizer *face.Recognizer
	Categories []string
}

func ReloadSamples(wrapper *RecognizerWrapper) error {
	if fis, err := GetFaces(); err != nil {
		return err
	} else {
		var descriptors []face.Descriptor
		var catIndexes []int32
		var categories []string
		for idx, faceInfo := range fis {
			descriptors = append(descriptors, faceInfo.Descriptor)
			catIndexes = append(catIndexes, int32(idx))
			categories = append(categories, faceInfo.Label)
		}
		wrapper.Recognizer.SetSamples(descriptors, catIndexes)
		wrapper.Categories = categories
		return nil
	}
}

func GetFaces() ([]FaceInfo, error) {
	if resp, err := http.Get("http://192.168.137.1:8080/api/device/rpi-00000000ece92c87/faceInfos"); err != nil {
		return nil, err
	} else {
		defer resp.Body.Close()
		fis := make([]FaceInfo, 0)
		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			return nil, err
		} else {
			err = json.Unmarshal(body, &fis)
			return fis, err
		}
	}
}
