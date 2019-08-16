package collection

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"archive/tar"
	"compress/gzip"
	"bytes"
	"io"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)


func DownloadToByte(url string) ([]byte, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not download file: %v", url))
	}
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	return b, err
}


//Read the manifests from a tar.gz archive
//It would be better to use the manifest.yaml as the index, and check the signatures
//For now, ignore manifest.yaml and return all other yaml files from the archive
func DecodeManifests (archive []byte) ([]unstructured.Unstructured, error) {

	manifests := []unstructured.Unstructured{}

	r := bytes.NewReader(archive)
	gzReader, err := gzip.NewReader(r)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not read manifest gzip"))
		}
	tarReader := tar.NewReader(gzReader)
	
	decoder := yaml.NewYAMLToJSONDecoder(tarReader)
	
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not read manifest tar"))
		}

		switch {
		case header.Name == "manifest.yaml":
			//ignore
		case strings.HasSuffix(header.Name, ".yaml"):
			out := unstructured.Unstructured{}
			err = decoder.Decode(&out)
			if err != nil {
				fmt.Sprintf("Error decoding %v", header.Name)
			}
			manifests = append(manifests, out)
		}
	}
	return manifests, nil
}

func GetManifests(url string) ([]unstructured.Unstructured, error) {
	b, err := DownloadToByte(url)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not download manifests archive"))
	}
	manifests, err := DecodeManifests(b)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Could not decode manifests archive"))
	}
	return manifests, err
}
