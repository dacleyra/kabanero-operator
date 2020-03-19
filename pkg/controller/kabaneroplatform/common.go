package kabaneroplatform

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/template"

	kabanerov1alpha2 "github.com/kabanero-io/kabanero-operator/pkg/apis/kabanero/v1alpha2"
	"github.com/kabanero-io/kabanero-operator/pkg/versioning"
)

// Evaluates the image uri using any provided overrides. Here repository, tag and image are from
// the Kabanero resource (the provided overrides). These values should be `` if no override is provided
// Precedence order is embedded data, repository/tag, and then image
func imageUriWithOverrides(repositoryOverride string, tagOverride string, imageOverride string, rev versioning.SoftwareRevision) (string, error) {
	image, err := customImageUriWithOverrides(repositoryOverride, tagOverride, imageOverride, rev, versioning.REPOSITORY_IDENTIFIER, versioning.TAG_IDENTIFIER)
	if err != nil {
		return "", err
	}

	return image, nil
}

func customImageUriWithOverrides(repositoryOverride string, tagOverride string, imageOverride string, rev versioning.SoftwareRevision, repoId string, tagId string) (string, error) {
	var r string
	var t string
	var i string

	// Start with embedded version specific data
	// Embedded data does not include an image as a single string, only repository and tag
	if v, ok := rev.Identifiers[repoId]; ok {
		if s, isString := v.(string); isString {
			r = s
		} else {
			return "", fmt.Errorf("The embedded identifier `%v` was expected to be a string", repoId)
		}

	}
	if v, ok := rev.Identifiers[tagId]; ok {
		if s, isString := v.(string); isString {
			t = s
		} else {
			return "", fmt.Errorf("The embedded identifier `%v` was expected to be a string", tagId)
		}
	}

	// TODO:
	// If r or t is "FROM_POD", then need to make sure that they are both "FROM_POD".
	// If "FROM_POD", make sure repositoryOverride and tagOverride are both set, or none are set.
	// Otherwise, business as usual.

	if ((r == "FROM_POD") || (t == "FROM_POD")) && (r != t) {
		return "", fmt.Errorf("Both repository and tag must be taken from the pod definition together")
	}

	if (r == "FROM_POD") {
		if bothZero(repositoryOverride, tagOverride) {
			if len(operatorContainerImage) == 0 { // defined in kabaneroplatform_controller.go
				return "", fmt.Errorf("This component cannot take its container image from the kabanero-operator pod because the kabanero-operator container image was not found")
			}
			
			i = operatorContainerImage
		} else if neitherZero(repositoryOverride, tagOverride) {
			i = repositoryOverride + ":" + tagOverride
		} else {
			return "", fmt.Errorf("This component requires both a repository and tag override.  Only one was provided")
		}	
  } else {
		// Next consider repository/tag from the Kabanero resource
		if repositoryOverride != "" {
			r = repositoryOverride
		}
		if tagOverride != "" {
			t = tagOverride
		}

		//repository/tag are now merged into image
		i = r + ":" + t
	}

	// Finally consider the image
	if imageOverride != "" {
		i = imageOverride
	}

	return i, nil
}

func renderOrchestration(r io.Reader, context map[string]interface{}) (string, error) {
	b, err := ioutil.ReadAll(r)
	templateText := string(b)

	t := template.Must(template.New("t1").
		Parse(templateText))

	var wr strings.Builder
	err = t.Execute(&wr, context)
	if err != nil {
		return "", err
	}
	rendered := wr.String()

	return rendered, nil
}

// Resolve the SoftwareRevision object for a named software component.
func resolveSoftwareRevision(k *kabanerov1alpha2.Kabanero, softwareComponent string, softwareVersionOverride string) (versioning.SoftwareRevision, error) {
	v, kabaneroVersion := resolveKabaneroVersion(k)

	kabaneroRevision := v.KabaneroRevision(kabaneroVersion)
	if kabaneroRevision == nil {
		return versioning.SoftwareRevision{}, fmt.Errorf("Data related to the Kabanero release identifier `%v` cannot be found", kabaneroVersion)
	}

	if softwareVersionOverride == "" {
		rev := kabaneroRevision.SoftwareComponent(softwareComponent)
		if rev == nil {
			return versioning.SoftwareRevision{}, fmt.Errorf("Data related to the software component `%v` within Kabanero release identifier `%v` cannot be found", softwareComponent, kabaneroVersion)
		}

		return *rev, nil
	} else {
		allRevs := v.RelatedSoftwareRevisions[softwareComponent]
		for _, rev := range allRevs {
			if rev.Version == softwareVersionOverride {
				return rev, nil
			}
		}

		return versioning.SoftwareRevision{}, fmt.Errorf("Data related to the software component `%v` and version `%v` within Kabanero release identifier `%v` cannot be found", softwareComponent, softwareVersionOverride, kabaneroVersion)
	}
}

// Resolves the version of the Kabanero instance.
func resolveKabaneroVersion(k *kabanerov1alpha2.Kabanero) (versioning.VersionDocument, string) {
	v := versioning.Data
	kabaneroVersion := k.Spec.Version
	if kabaneroVersion == "" {
		kabaneroVersion = v.DefaultKabaneroRevision
	}
  return v, kabaneroVersion
}

func bothZero(string1 string, string2 string) bool {
	return (len(string1) == 0) && (len(string2) == 0)
}

func neitherZero(string1 string, string2 string) bool {
	return (len(string1) > 0) && (len(string2) > 0)
}
