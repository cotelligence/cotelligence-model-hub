package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Helper function to resolve $ref in the schema and its properties recursively
func resolveRefs(schemaRef *openapi3.SchemaRef, swagger *openapi3.T) *openapi3.SchemaRef {
	for schemaRef.Ref != "" {
		schemaKey := strings.TrimPrefix(schemaRef.Ref, "#/components/schemas/")
		schemaRef = swagger.Components.Schemas[schemaKey]
	}
	if schemaRef.Value != nil {
		for propertyName, property := range schemaRef.Value.Properties {
			schemaRef.Value.Properties[propertyName] = resolveRefs(property, swagger)
		}
	}
	return schemaRef
}

func GetSampleIO(modelUUID string) (string, string, error) {
	// Read the OpenAPI JSON file
	data, err := os.ReadFile(fmt.Sprintf("model_spec/%s.json", modelUUID))
	if err != nil {
		return "", "", fmt.Errorf("failed to read OpenAPI JSON file: %w", err)
	}

	// Unmarshal the JSON into an openapi3.T object
	loader := openapi3.NewLoader()
	swagger, err := loader.LoadFromData(data)
	if err != nil {
		return "", "", fmt.Errorf("failed to unmarshal OpenAPI JSON: %w", err)
	}

	// Use JSONLookup to find the path item for "/predictions"
	paths := swagger.Paths
	pathItem := paths.Value("/predictions")
	// Access its operations (like Get, Post, etc.)
	operation := pathItem.GetOperation(http.MethodPost)
	// Access the request body to get the example input
	var reqExample, resExample string
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		for _, mediaType := range operation.RequestBody.Value.Content {
			if mediaType.Example != nil {
				reqExampleBytes, err := json.Marshal(mediaType.Example)
				if err != nil {
					return "", "", fmt.Errorf("failed to marshal request example: %w", err)
				}
				reqExample = string(reqExampleBytes)
			} else if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				schemaRef := mediaType.Schema
				// Resolve $ref in the schema and its properties
				schemaRef = resolveRefs(schemaRef, swagger)
				if schemaRef.Value != nil {
					// Create a map to hold the properties
					propertiesMap := make(map[string]interface{})
					// Iterate over the properties of the schema
					for propertyName, property := range schemaRef.Value.Properties {
						if property.Value != nil && property.Value.Default != nil {
							propertiesMap[propertyName] = property.Value.Default
						} else {
							// check if the property has properties
							if property.Value != nil && property.Value.Properties != nil {
								// Create a map to hold the sub-properties
								innerProps := make(map[string]interface{})
								// Iterate over the sub-properties
								for innerPropName, innerProp := range property.Value.Properties {
									if innerProp.Value != nil && innerProp.Value.Default != nil {
										innerProps[innerPropName] = innerProp.Value.Default
									} else {
										innerProps[innerPropName] = ""
									}
								}
								propertiesMap[propertyName] = innerProps
							} else {
								propertiesMap[propertyName] = ""
							}
						}
					}
					// Marshal the properties map to a JSON string
					reqExampleBytes, err := json.Marshal(propertiesMap)
					if err != nil {
						return "", "", fmt.Errorf("failed to marshal request properties: %w", err)
					}
					reqExample = string(reqExampleBytes)
				}
			}
			break // Assuming we only need one example
		}
	}

	// Access the responses to get the example output
	response := operation.Responses.Value("200")
	if response.Value != nil {
		for _, mediaType := range response.Value.Content {
			if mediaType.Example != nil {
				resExampleBytes, err := json.Marshal(mediaType.Example)
				if err != nil {
					return "", "", fmt.Errorf("failed to marshal response example: %w", err)
				}
				resExample = string(resExampleBytes)
			} else if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				schemaRef := mediaType.Schema
				// Resolve $ref in the schema and its properties
				schemaRef = resolveRefs(schemaRef, swagger)
				if schemaRef.Value != nil {
					// Create a map to hold the properties
					propertiesMap := make(map[string]interface{})
					// Iterate over the properties of the schema
					for propertyName, property := range schemaRef.Value.Properties {
						if property.Value != nil && property.Value.Default != nil {
							propertiesMap[propertyName] = property.Value.Default
						} else {
							// check if the property has properties
							if property.Value != nil && property.Value.Properties != nil {
								// Create a map to hold the sub-properties
								innerProps := make(map[string]interface{})
								// Iterate over the sub-properties
								for innerPropName, innerProp := range property.Value.Properties {
									if innerProp.Value != nil && innerProp.Value.Default != nil {
										innerProps[innerPropName] = innerProp.Value.Default
									} else {
										innerProps[innerPropName] = ""
									}
								}
								propertiesMap[propertyName] = innerProps
							} else {
								propertiesMap[propertyName] = ""
							}
						}
					}
					// Marshal the properties map to a JSON string
					resExampleBytes, err := json.Marshal(propertiesMap)
					if err != nil {
						return "", "", fmt.Errorf("failed to marshal response properties: %w", err)
					}
					resExample = string(resExampleBytes)
				}
			}
			break // Assuming we only need one example
		}
	}

	if reqExample != "" && resExample != "" {
		return reqExample, resExample, nil
	}

	return "", "", fmt.Errorf("no examples found for /predictions")
}
