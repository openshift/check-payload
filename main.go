package main

import (
	"encoding/json"
	"log"
	"os"

	corev1 "k8s.io/api/core/v1"
)

type ArtifactPod struct {
	ApiVersion string       `json:"apiVersion"`
	Items      []corev1.Pod `json:"items"`
}

func ReadArtifactPods(filename string) (*ArtifactPod, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	apod := &ArtifactPod{}
	if err := json.Unmarshal([]byte(data), &apod); err != nil {
		return nil, err
	}
	return apod, nil
}

func main() {
	apods, err := ReadArtifactPods("pods.json")
	if err != nil {
		log.Fatal(err)
	}
	for _, pod := range apods.Items {
		for _, container := range pod.Spec.Containers {
			if len(container.Command) == 0 {
				// need to use entrypoint of image
				continue
			}
			log.Printf("%v %v %v %v", pod.Name, container.Name, container.Image, container.Command[0])
		}
	}
}
