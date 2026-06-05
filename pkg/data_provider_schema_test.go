package pkg_test

import (
	"context"
	"encoding/json"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	"math/big"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestDataProviderSchema_ConvertTFJsonSchemaToCtyValue(t *testing.T) {
	schemaJson := `{
  "format_version": "1.0",
  "provider_schemas": {
    "registry.terraform.io/hashicorp/local": {
      "provider": {
        "version": 0,
        "block": {
          "description_kind": "plain"
        }
      },
      "resource_schemas": {
        "local_file": {
          "version": 0,
          "block": {
            "attributes": {
              "content": {
                "type": "string",
                "description": "Content to store in the file, expected to be a UTF-8 encoded string.",
                "description_kind": "plain",
                "optional": true
              },
              "content_base64": {
                "type": "string",
                "description": "Content to store in the file, expected to be binary encoded as base64 string.",
                "description_kind": "plain",
                "optional": true
              },
              "content_base64sha256": {
                "type": "string",
                "description": "Base64 encoded SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64sha512": {
                "type": "string",
                "description": "Base64 encoded SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_md5": {
                "type": "string",
                "description": "MD5 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha1": {
                "type": "string",
                "description": "SHA1 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha256": {
                "type": "string",
                "description": "SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha512": {
                "type": "string",
                "description": "SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "directory_permission": {
                "type": "string",
                "description": "Permissions to set for directories created (before umask), expressed as string in",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "file_permission": {
                "type": "string",
                "description": "Permissions to set for the output file (before umask), expressed as string in",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "filename": {
                "type": "string",
                "description": "The path to the file that will be created.",
                "description_kind": "plain",
                "required": true
              },
              "id": {
                "type": "string",
                "description": "The hexadecimal encoding of the SHA1 checksum of the file content.",
                "description_kind": "plain",
                "computed": true
              },
              "sensitive_content": {
                "type": "string",
                "description": "Sensitive content to store in the file, expected to be an UTF-8 encoded string.",
                "description_kind": "plain",
                "deprecated": true,
                "optional": true,
                "sensitive": true
              },
              "source": {
                "type": "string",
                "description": "Path to file to use as source for the one we are creating.",
                "description_kind": "plain",
                "optional": true
              }
            },
            "description": "Generates a local file with the given content.",
            "description_kind": "plain"
          }
        }
      }
    }
  }
}
`
	var schema tfjson.ProviderSchemas
	require.NoError(t, json.Unmarshal([]byte(schemaJson), &schema))
	providerSchema, ok := schema.Schemas["registry.terraform.io/hashicorp/local"]
	require.True(t, ok)
	sut := &pkg.ProviderSchemaData{}
	actual, err := sut.Convert(providerSchema.ResourceSchemas)
	require.NoError(t, err)
	expected := cty.ObjectVal(map[string]cty.Value{
		"local_file": cty.ObjectVal(map[string]cty.Value{
			"version": cty.NumberIntVal(0),
			"block": cty.ObjectVal(map[string]cty.Value{
				"attributes": cty.ObjectVal(map[string]cty.Value{
					"content": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`Content to store in the file, expected to be a UTF-8 encoded string.`),
						"description_kind": cty.StringVal("plain"),
						"optional":         cty.True,
					}),
					"content_base64": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`Content to store in the file, expected to be binary encoded as base64 string.`),
						"description_kind": cty.StringVal("plain"),
						"optional":         cty.True,
					}),
					"content_base64sha256": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("Base64 encoded SHA256 checksum of file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"content_base64sha512": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("Base64 encoded SHA512 checksum of file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"content_md5": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("MD5 checksum of file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"content_sha1": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("SHA1 checksum of file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"content_sha256": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("SHA256 checksum of file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"content_sha512": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("SHA512 checksum of file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"directory_permission": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`Permissions to set for directories created (before umask), expressed as string in`),
						"description_kind": cty.StringVal("plain"),
						"optional":         cty.True,
						"computed":         cty.True,
					}),
					"file_permission": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`Permissions to set for the output file (before umask), expressed as string in`),
						"description_kind": cty.StringVal("plain"),
						"optional":         cty.True,
						"computed":         cty.True,
					}),
					"filename": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`The path to the file that will be created.`),
						"description_kind": cty.StringVal("plain"),
						"required":         cty.True,
					}),
					"id": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal("The hexadecimal encoding of the SHA1 checksum of the file content."),
						"description_kind": cty.StringVal("plain"),
						"computed":         cty.True,
					}),
					"sensitive_content": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`Sensitive content to store in the file, expected to be an UTF-8 encoded string.`),
						"description_kind": cty.StringVal("plain"),
						"deprecated":       cty.True,
						"sensitive":        cty.True,
						"optional":         cty.True,
					}),
					"source": cty.ObjectVal(map[string]cty.Value{
						"type":             cty.StringVal("string"),
						"description":      cty.StringVal(`Path to file to use as source for the one we are creating.`),
						"description_kind": cty.StringVal("plain"),
						"optional":         cty.True,
					}),
				}),
				"block_types": cty.EmptyObjectVal,
				"description": cty.StringVal("Generates a local file with the given content."),
			}),
		}),
	})
	assert.Equal(t, expected, actual)
}

func TestDataProviderSchema_ConvertAttributeSchemaToCtyValue(t *testing.T) {
	cases := []struct {
		schema      *tfjson.SchemaAttribute
		expected    cty.Value
		description string
	}{
		{
			schema: &tfjson.SchemaAttribute{
				AttributeType: cty.String,
				Description:   "description",
				Deprecated:    true,
				Required:      true,
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"type":        cty.StringVal("string"),
				"description": cty.StringVal("description"),
				"deprecated":  cty.BoolVal(true),
				"required":    cty.True,
			}),
			description: "simple required string attribute",
		},
		{
			schema: &tfjson.SchemaAttribute{
				AttributeType: cty.String,
				Description:   "description",
				Deprecated:    true,
				Optional:      true,
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"type":        cty.StringVal("string"),
				"description": cty.StringVal("description"),
				"deprecated":  cty.True,
				"optional":    cty.True,
			}),
			description: "simple optional string attribute",
		},
		{
			description: "list of primitive type",
			schema: &tfjson.SchemaAttribute{
				AttributeType: cty.List(cty.String),
				Description:   "list_of_string",
				Optional:      true,
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"type":        mustDecode(t, `["list", "string"]`),
				"description": cty.StringVal("list_of_string"),
				"optional":    cty.True,
			}),
		},
		{
			description: "list of object",
			schema: &tfjson.SchemaAttribute{
				AttributeType: cty.List(cty.Object(map[string]cty.Type{
					"connection_string": cty.String,
					"id":                cty.String,
					"secret":            cty.String,
				})),
				Description: "list_of_object",
				Optional:    true,
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"type":        mustDecode(t, `["list",["object",{"connection_string": "string","id": "string","secret": "string"}]]`),
				"description": cty.StringVal("list_of_object"),
				"optional":    cty.True,
			}),
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			sut := &pkg.ProviderSchemaData{}
			actual, err := sut.Convert(map[string]*tfjson.Schema{
				"test": {
					Block: &tfjson.SchemaBlock{
						Attributes: map[string]*tfjson.SchemaAttribute{
							"test_attr": c.schema,
						},
					},
				},
			})
			require.NoError(t, err)
			attr := actual.GetAttr("test").GetAttr("block").GetAttr("attributes").GetAttr("test_attr")
			assert.Equal(t, c.expected, attr)
		})
	}
}

func TestDataProviderSchema_ConvertNestedBlockSchemaToCtyValue(t *testing.T) {
	one := big.NewFloat(1.0)
	one.SetPrec(512)
	cases := []struct {
		schema      *tfjson.SchemaBlockType
		expected    cty.Value
		description string
	}{
		{
			description: "simple",
			schema: &tfjson.SchemaBlockType{
				NestingMode: tfjson.SchemaNestingModeList,
				MaxItems:    1,
				Block: &tfjson.SchemaBlock{
					Attributes: map[string]*tfjson.SchemaAttribute{
						"identity_client_id": {
							AttributeType:   cty.String,
							DescriptionKind: tfjson.SchemaDescriptionKindPlain,
							Optional:        true,
						},
						"key_vault_key_identifier": {
							AttributeType:   cty.String,
							DescriptionKind: tfjson.SchemaDescriptionKindPlain,
							Optional:        true,
						},
					},
				},
			},
			expected: cty.ObjectVal(map[string]cty.Value{
				"nesting_mode": cty.StringVal("list"),
				"max_items":    cty.NumberVal(one),
				"block": cty.ObjectVal(map[string]cty.Value{
					"attributes": cty.ObjectVal(map[string]cty.Value{
						"identity_client_id": cty.ObjectVal(map[string]cty.Value{
							"type":             cty.StringVal("string"),
							"description_kind": cty.StringVal("plain"),
							"optional":         cty.True,
						}),
						"key_vault_key_identifier": cty.ObjectVal(map[string]cty.Value{
							"type":             cty.StringVal("string"),
							"description_kind": cty.StringVal("plain"),
							"optional":         cty.True,
						}),
					}),
				}),
			}),
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			sut := &pkg.ProviderSchemaData{}
			actual, err := sut.Convert(map[string]*tfjson.Schema{
				"test": {
					Block: &tfjson.SchemaBlock{
						NestedBlocks: map[string]*tfjson.SchemaBlockType{
							"block": c.schema,
						},
					},
				},
			})
			require.NoError(t, err)
			nb := actual.GetAttr("test").GetAttr("block").GetAttr("block_types").GetAttr("block")
			assert.Equal(t, c.expected, nb)
		})
	}
}

func TestDataProviderSchema_mockSchemaRetriever(t *testing.T) {
	localSchema := `{
      "provider": {
        "version": 0,
        "block": {
          "description_kind": "plain"
        }
      },
      "resource_schemas": {
        "azurerm_app_configuration": {
          "version": 0,
          "block": {
            "attributes": {
              "endpoint": {
                "type": "string",
                "description_kind": "plain",
                "computed": true
              },
              "id": {
                "type": "string",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "local_auth_enabled": {
                "type": "bool",
                "description_kind": "plain",
                "optional": true
              },
              "location": {
                "type": "string",
                "description_kind": "plain",
                "required": true
              },
              "name": {
                "type": "string",
                "description_kind": "plain",
                "required": true
              },
              "primary_read_key": {
                "type": [
                  "list",
                  [
                    "object",
                    {
                      "connection_string": "string",
                      "id": "string",
                      "secret": "string"
                    }
                  ]
                ],
                "description_kind": "plain",
                "computed": true
              },
              "primary_write_key": {
                "type": [
                  "list",
                  [
                    "object",
                    {
                      "connection_string": "string",
                      "id": "string",
                      "secret": "string"
                    }
                  ]
                ],
                "description_kind": "plain",
                "computed": true
              },
              "public_network_access": {
                "type": "string",
                "description_kind": "plain",
                "optional": true
              },
              "purge_protection_enabled": {
                "type": "bool",
                "description_kind": "plain",
                "optional": true
              },
              "resource_group_name": {
                "type": "string",
                "description_kind": "plain",
                "required": true
              },
              "secondary_read_key": {
                "type": [
                  "list",
                  [
                    "object",
                    {
                      "connection_string": "string",
                      "id": "string",
                      "secret": "string"
                    }
                  ]
                ],
                "description_kind": "plain",
                "computed": true
              },
              "secondary_write_key": {
                "type": [
                  "list",
                  [
                    "object",
                    {
                      "connection_string": "string",
                      "id": "string",
                      "secret": "string"
                    }
                  ]
                ],
                "description_kind": "plain",
                "computed": true
              },
              "sku": {
                "type": "string",
                "description_kind": "plain",
                "optional": true
              },
              "soft_delete_retention_days": {
                "type": "number",
                "description_kind": "plain",
                "optional": true
              },
              "tags": {
                "type": [
                  "map",
                  "string"
                ],
                "description_kind": "plain",
                "optional": true
              }
            },
            "block_types": {
              "encryption": {
                "nesting_mode": "list",
                "block": {
                  "attributes": {
                    "identity_client_id": {
                      "type": "string",
                      "description_kind": "plain",
                      "optional": true
                    },
                    "key_vault_key_identifier": {
                      "type": "string",
                      "description_kind": "plain",
                      "optional": true
                    }
                  },
                  "description_kind": "plain"
                },
                "max_items": 1
              },
              "identity": {
                "nesting_mode": "list",
                "block": {
                  "attributes": {
                    "identity_ids": {
                      "type": [
                        "set",
                        "string"
                      ],
                      "description_kind": "plain",
                      "optional": true
                    },
                    "principal_id": {
                      "type": "string",
                      "description_kind": "plain",
                      "computed": true
                    },
                    "tenant_id": {
                      "type": "string",
                      "description_kind": "plain",
                      "computed": true
                    },
                    "type": {
                      "type": "string",
                      "description_kind": "plain",
                      "required": true
                    }
                  },
                  "description_kind": "plain"
                },
                "max_items": 1
              },
              "replica": {
                "nesting_mode": "set",
                "block": {
                  "attributes": {
                    "endpoint": {
                      "type": "string",
                      "description_kind": "plain",
                      "computed": true
                    },
                    "id": {
                      "type": "string",
                      "description_kind": "plain",
                      "computed": true
                    },
                    "location": {
                      "type": "string",
                      "description_kind": "plain",
                      "required": true
                    },
                    "name": {
                      "type": "string",
                      "description_kind": "plain",
                      "required": true
                    }
                  },
                  "description_kind": "plain"
                }
              },
              "timeouts": {
                "nesting_mode": "single",
                "block": {
                  "attributes": {
                    "create": {
                      "type": "string",
                      "description_kind": "plain",
                      "optional": true
                    },
                    "delete": {
                      "type": "string",
                      "description_kind": "plain",
                      "optional": true
                    },
                    "read": {
                      "type": "string",
                      "description_kind": "plain",
                      "optional": true
                    },
                    "update": {
                      "type": "string",
                      "description_kind": "plain",
                      "optional": true
                    }
                  },
                  "description_kind": "plain"
                }
              }
            },
            "description_kind": "plain"
          }
        },
        "local_file": {
          "version": 0,
          "block": {
            "attributes": {
              "content": {
                "type": "string",
                "description": "Content to store in the file, expected to be a UTF-8 encoded string. Conflicts with 'sensitive_content', 'content_base64' and 'source'. Exactly one of these four arguments must be specified.",
                "description_kind": "plain",
                "optional": true
              },
              "content_base64": {
                "type": "string",
                "description": "Content to store in the file, expected to be binary encoded as base64 string. Conflicts with 'content', 'sensitive_content' and 'source'. Exactly one of these four arguments must be specified.",
                "description_kind": "plain",
                "optional": true
              },
              "content_base64sha256": {
                "type": "string",
                "description": "Base64 encoded SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64sha512": {
                "type": "string",
                "description": "Base64 encoded SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_md5": {
                "type": "string",
                "description": "MD5 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha1": {
                "type": "string",
                "description": "SHA1 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha256": {
                "type": "string",
                "description": "SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha512": {
                "type": "string",
                "description": "SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "directory_permission": {
                "type": "string",
                "description": "Permissions to set for directories created (before umask), expressed as string in [numeric notation](https://en.wikipedia.org/wiki/File-system_permissions#Numeric_notation). Default value is '\"0777\"'.",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "file_permission": {
                "type": "string",
                "description": "Permissions to set for the output file (before umask), expressed as string in [numeric notation](https://en.wikipedia.org/wiki/File-system_permissions#Numeric_notation). Default value is '\"0777\"'.",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "filename": {
                "type": "string",
                "description": "The path to the file that will be created. Missing parent directories will be created. If the file already exists, it will be overridden with the given content.",
                "description_kind": "plain",
                "required": true
              },
              "id": {
                "type": "string",
                "description": "The hexadecimal encoding of the SHA1 checksum of the file content.",
                "description_kind": "plain",
                "computed": true
              },
              "sensitive_content": {
                "type": "string",
                "description": "Sensitive content to store in the file, expected to be an UTF-8 encoded string. Will not be displayed in diffs. Conflicts with 'content', 'content_base64' and 'source'. Exactly one of these four arguments must be specified. If in need to use _sensitive_ content, please use the ['local_sensitive_file'](./sensitive_file.html) resource instead.",
                "description_kind": "plain",
                "deprecated": true,
                "optional": true,
                "sensitive": true
              },
              "source": {
                "type": "string",
                "description": "Path to file to use as source for the one we are creating. Conflicts with 'content', 'sensitive_content' and 'content_base64'. Exactly one of these four arguments must be specified.",
                "description_kind": "plain",
                "optional": true
              }
            },
            "description": "Generates a local file with the given content.",
            "description_kind": "plain"
          }
        },
        "local_sensitive_file": {
          "version": 0,
          "block": {
            "attributes": {
              "content": {
                "type": "string",
                "description": "Sensitive Content to store in the file, expected to be a UTF-8 encoded string. Conflicts with 'content_base64' and 'source'. Exactly one of these three arguments must be specified.",
                "description_kind": "plain",
                "optional": true,
                "sensitive": true
              },
              "content_base64": {
                "type": "string",
                "description": "Sensitive Content to store in the file, expected to be binary encoded as base64 string. Conflicts with 'content' and 'source'. Exactly one of these three arguments must be specified.",
                "description_kind": "plain",
                "optional": true,
                "sensitive": true
              },
              "content_base64sha256": {
                "type": "string",
                "description": "Base64 encoded SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64sha512": {
                "type": "string",
                "description": "Base64 encoded SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_md5": {
                "type": "string",
                "description": "MD5 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha1": {
                "type": "string",
                "description": "SHA1 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha256": {
                "type": "string",
                "description": "SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha512": {
                "type": "string",
                "description": "SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "directory_permission": {
                "type": "string",
                "description": "Permissions to set for directories created (before umask), expressed as string in [numeric notation](https://en.wikipedia.org/wiki/File-system_permissions#Numeric_notation). Default value is '\"0700\"'.",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "file_permission": {
                "type": "string",
                "description": "Permissions to set for the output file (before umask), expressed as string in [numeric notation](https://en.wikipedia.org/wiki/File-system_permissions#Numeric_notation). Default value is '\"0700\"'.",
                "description_kind": "plain",
                "optional": true,
                "computed": true
              },
              "filename": {
                "type": "string",
                "description": "The path to the file that will be created. Missing parent directories will be created. If the file already exists, it will be overridden with the given content.",
                "description_kind": "plain",
                "required": true
              },
              "id": {
                "type": "string",
                "description": "The hexadecimal encoding of the SHA1 checksum of the file content.",
                "description_kind": "plain",
                "computed": true
              },
              "source": {
                "type": "string",
                "description": "Path to file to use as source for the one we are creating. Conflicts with 'content' and 'content_base64'. Exactly one of these three arguments must be specified.",
                "description_kind": "plain",
                "optional": true
              }
            },
            "description": "Generates a local file with the given sensitive content.",
            "description_kind": "plain"
          }
        }
      },
      "data_source_schemas": {
        "local_file": {
          "version": 0,
          "block": {
            "attributes": {
              "content": {
                "type": "string",
                "description": "Raw content of the file that was read, as UTF-8 encoded string. Files that do not contain UTF-8 text will have invalid UTF-8 sequences in 'content'  replaced with the Unicode replacement character. ",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64": {
                "type": "string",
                "description": "Base64 encoded version of the file content (use this when dealing with binary data).",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64sha256": {
                "type": "string",
                "description": "Base64 encoded SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64sha512": {
                "type": "string",
                "description": "Base64 encoded SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_md5": {
                "type": "string",
                "description": "MD5 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha1": {
                "type": "string",
                "description": "SHA1 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha256": {
                "type": "string",
                "description": "SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha512": {
                "type": "string",
                "description": "SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "filename": {
                "type": "string",
                "description": "Path to the file that will be read. The data source will return an error if the file does not exist.",
                "description_kind": "plain",
                "required": true
              },
              "id": {
                "type": "string",
                "description": "The hexadecimal encoding of the SHA1 checksum of the file content.",
                "description_kind": "plain",
                "computed": true
              }
            },
            "description": "Reads a file from the local filesystem.",
            "description_kind": "plain"
          }
        },
        "local_sensitive_file": {
          "version": 0,
          "block": {
            "attributes": {
              "content": {
                "type": "string",
                "description": "Raw content of the file that was read, as UTF-8 encoded string. Files that do not contain UTF-8 text will have invalid UTF-8 sequences in 'content'  replaced with the Unicode replacement character.",
                "description_kind": "plain",
                "computed": true,
                "sensitive": true
              },
              "content_base64": {
                "type": "string",
                "description": "Base64 encoded version of the file content (use this when dealing with binary data).",
                "description_kind": "plain",
                "computed": true,
                "sensitive": true
              },
              "content_base64sha256": {
                "type": "string",
                "description": "Base64 encoded SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_base64sha512": {
                "type": "string",
                "description": "Base64 encoded SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_md5": {
                "type": "string",
                "description": "MD5 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha1": {
                "type": "string",
                "description": "SHA1 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha256": {
                "type": "string",
                "description": "SHA256 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "content_sha512": {
                "type": "string",
                "description": "SHA512 checksum of file content.",
                "description_kind": "plain",
                "computed": true
              },
              "filename": {
                "type": "string",
                "description": "Path to the file that will be read. The data source will return an error if the file does not exist.",
                "description_kind": "plain",
                "required": true
              },
              "id": {
                "type": "string",
                "description": "The hexadecimal encoding of the SHA1 checksum of the file content.",
                "description_kind": "plain",
                "computed": true
              }
            },
            "description": "Reads a file that contains sensitive data, from the local filesystem.",
            "description_kind": "plain"
          }
        }
      },
      "functions": {
        "direxists": {
          "description": "Given a path string, will return true if the directory exists. This function works only with directories. If used with a file, the function will return an error.This function behaves similar to the built-in ['fileexists'](https://developer.hashicorp.com/terraform/language/functions/fileexists) function, however, 'direxists' will not replace filesystem paths including '~' with the current user's home directory path. This functionality can be achieved by using the built-in ['pathexpand'](https://developer.hashicorp.com/terraform/language/functions/pathexpand) function with 'direxists', see example below.",
          "summary": "Determines whether a directory exists at a given path.",
          "return_type": "bool",
          "parameters": [
            {
              "name": "path",
              "description": "Relative or absolute path to check for the existence of a directory",
              "type": "string"
            }
          ]
        }
      }
    }
`
	stub := gostub.Stub(&pkg.SchemaRetrieverFactory, func(ctx context.Context) pkg.TerraformProviderSchemaRetriever {
		return mockProviderSchemaRetriever{t: t, jsonSchema: localSchema}
	}).Stub(&filesystem.Fs, fakeFs(map[string]string{
		filepath.Join("terraform", "main.tf"): `resource "azurerm_app_configuration" this {
}

resource "azurerm_non_exist" this {
}
`,
		filepath.Join("mptf", "main.mptf.hcl"): `data "resource" all {
}

data "provider_schema" this {
  provider_source = "Azure/fake"
  provider_version = ">= 0.1.0"
}

locals {
  resources_support_tags = toset([ for name, r in data.provider_schema.this.resources : name if try(r.block.attributes["tags"].type == ["map", "string"], false) ])
  resource_apply_tags = flatten([ for resource_type, resource_blocks in data.resource.all.result : resource_blocks if contains(local.resources_support_tags, resource_type) ])
  mptfs = flatten([for _, blocks in local.resource_apply_tags : [for b in blocks : b.mptf]])
  addresses = [for mptf in local.mptfs : mptf.block_address]
}

transform "update_in_place" tags {
  for_each = try(local.addresses, [])
  target_block_address = each.value
  
  asraw {
    tags = {
      hello = "world"
    }
  }
}
`,
	}))
	defer stub.Reset()
	hclBlocks, err := pkg.LoadMPTFHclBlocks(false, "mptf")
	require.NoError(t, err)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "terraform",
		AbsDir: "terraform",
	}, nil, hclBlocks, nil, context.TODO())
	require.NoError(t, err)
	plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
	require.NoError(t, err)
	assert.Len(t, plan.Transforms, 1)
	assert.Equal(t, "resource.azurerm_app_configuration.this", plan.Transforms[0].(*pkg.UpdateInPlaceTransform).TargetBlockAddress)
}

var _ pkg.TerraformProviderSchemaRetriever = &mockProviderSchemaRetriever{}

type mockProviderSchemaRetriever struct {
	t          *testing.T
	jsonSchema string
}

func (m mockProviderSchemaRetriever) Get(providerSource, versionConstraint string) (*tfjson.ProviderSchema, error) {
	var schema tfjson.ProviderSchema
	require.NoError(m.t, json.Unmarshal([]byte(m.jsonSchema), &schema))
	return &schema, nil
}

func mustDecode(t *testing.T, s string) cty.Value {
	v, err := stdlib.JSONDecode(cty.StringVal(s))
	require.NoError(t, err)
	return v
}

// TestDataProviderSchema_SortedAttributeLists exercises the four pre-sorted
// attribute-name lists added in v0.1.4 (resources_required_attributes,
// resources_optional_attributes, data_sources_required_attributes,
// data_sources_optional_attributes) plus the new data_sources attribute. The
// synthetic schema deliberately covers the four tfjson partitions so the
// `Required` / `Optional` filter behaviour is locked in:
//
//   - Required: true                       → in *_required, not in *_optional
//   - Optional: true                       → in *_optional, not in *_required
//   - Optional: true, Computed: true       → in *_optional (user-writable)
//   - Computed: true (alone)               → in neither (pure computed)
//
// Each list element is fanned out into one update_in_place transform via
// for_each, where the for_each key is the original list index (zero-padded so
// alphabetical key sort preserves the original order) and the asstring body
// records the attribute name. The test then walks `cfg.GetVertices()` to read
// the patch block for each generated transform and assert the recovered names
// match the expected alphabetical ordering.
func TestDataProviderSchema_SortedAttributeLists(t *testing.T) {
	syntheticSchema := `{
      "provider": {
        "version": 0,
        "block": {"description_kind": "plain"}
      },
      "resource_schemas": {
        "azurerm_resource_group": {
          "version": 0,
          "block": {
            "attributes": {
              "name":              {"type": "string", "required": true},
              "location":          {"type": "string", "required": true},
              "managed_by":        {"type": "string", "optional": true},
              "tags":              {"type": ["map", "string"], "optional": true},
              "id":                {"type": "string", "optional": true, "computed": true},
              "etag":              {"type": "string", "computed": true}
            },
            "description_kind": "plain"
          }
        },
        "azurerm_kv_certificate": {
          "version": 0,
          "block": {
            "attributes": {
              "key_vault_id":      {"type": "string", "required": true},
              "name":              {"type": "string", "required": true},
              "tags":              {"type": ["map", "string"], "optional": true},
              "version":           {"type": "string", "computed": true}
            },
            "description_kind": "plain"
          }
        }
      },
      "data_source_schemas": {
        "azurerm_resource_group": {
          "version": 0,
          "block": {
            "attributes": {
              "name":              {"type": "string", "required": true},
              "location":          {"type": "string", "computed": true},
              "tags":              {"type": ["map", "string"], "computed": true},
              "id":                {"type": "string", "optional": true, "computed": true}
            },
            "description_kind": "plain"
          }
        }
      }
    }
`
	stub := gostub.Stub(&pkg.SchemaRetrieverFactory, func(ctx context.Context) pkg.TerraformProviderSchemaRetriever {
		return mockProviderSchemaRetriever{t: t, jsonSchema: syntheticSchema}
	}).Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `resource "azurerm_resource_group" this {
}
`,
	}))
	defer stub.Reset()

	hclBlocks := newHclBlocks(t, `
data "provider_schema" this {
  provider_source  = "Azure/fake"
  provider_version = ">= 0.1.0"
}

transform "update_in_place" rg_required {
  for_each             = { for i, n in data.provider_schema.this.resources_required_attributes["azurerm_resource_group"] : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" rg_optional {
  for_each             = { for i, n in data.provider_schema.this.resources_optional_attributes["azurerm_resource_group"] : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" kv_required {
  for_each             = { for i, n in data.provider_schema.this.resources_required_attributes["azurerm_kv_certificate"] : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" ds_rg_required {
  for_each             = { for i, n in data.provider_schema.this.data_sources_required_attributes["azurerm_resource_group"] : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" ds_rg_optional {
  for_each             = { for i, n in data.provider_schema.this.data_sources_optional_attributes["azurerm_resource_group"] : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}

transform "update_in_place" data_sources_marker {
  for_each             = { for i, n in sort(keys(data.provider_schema.this.data_sources)) : format("%04d", i) => n }
  target_block_address = "resource.azurerm_resource_group.this"
  asstring {
    description = each.value
  }
}
`)
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    ".",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)
	require.NoError(t, cfg.Init(hclBlocks))
	require.NoError(t, cfg.RunPrePlan())
	require.NoError(t, cfg.RunPlan())

	got := collectForEachStrings(t, cfg, "update_in_place", []string{
		"rg_required",
		"rg_optional",
		"kv_required",
		"ds_rg_required",
		"ds_rg_optional",
		"data_sources_marker",
	}, "description")

	// azurerm_resource_group: required = [location, name] (alphabetical).
	assert.Equal(t, []string{"location", "name"}, got["rg_required"])
	// azurerm_resource_group: optional = [id, managed_by, tags]; id is
	// optional+computed (still user-writable), etag is pure-computed (excluded).
	assert.Equal(t, []string{"id", "managed_by", "tags"}, got["rg_optional"])
	// azurerm_kv_certificate: required = [key_vault_id, name]; version is
	// pure-computed (excluded from both buckets).
	assert.Equal(t, []string{"key_vault_id", "name"}, got["kv_required"])
	// data source azurerm_resource_group: required = [name].
	assert.Equal(t, []string{"name"}, got["ds_rg_required"])
	// data source azurerm_resource_group: optional = [id] (id is
	// optional+computed; location and tags are pure-computed).
	assert.Equal(t, []string{"id"}, got["ds_rg_optional"])
	// `data_sources` exposes every data source type from the schema.
	assert.Equal(t, []string{"azurerm_resource_group"}, got["data_sources_marker"])
}

// collectForEachStrings walks the for_each transform vertices produced by
// `transform.<transformType>.<baseName>[<key>]` and, for each baseName, returns
// the ordered slice of values pulled from the named asstring attribute on each
// generated patch block. Order is by for_each key (zero-padded indexes sort
// lexicographically into original list order).
func collectForEachStrings(t *testing.T, cfg *pkg.MetaProgrammingTFConfig, transformType string, baseNames []string, attrName string) map[string][]string {
	t.Helper()
	vertices := cfg.GetVertices()
	out := make(map[string][]string, len(baseNames))
	for _, base := range baseNames {
		prefix := "transform." + transformType + "." + base + "["
		keys := make([]string, 0)
		entries := make(map[string]string)
		for vk, vb := range vertices {
			if !strings.HasPrefix(vk, prefix) || !strings.HasSuffix(vk, "]") {
				continue
			}
			forEachKey := strings.TrimSuffix(strings.TrimPrefix(vk, prefix), "]")
			upd, ok := vb.(*pkg.UpdateInPlaceTransform)
			require.Truef(t, ok, "vertex %q is not an UpdateInPlaceTransform", vk)
			attr := upd.UpdateBlock().Body().GetAttribute(attrName)
			require.NotNilf(t, attr, "vertex %q has no %q attribute on its patch block", vk, attrName)
			rawTokens := attr.Expr().BuildTokens(nil).Bytes()
			value := strings.TrimSpace(string(rawTokens))
			// Strip enclosing quotes from the asstring-rendered literal.
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			keys = append(keys, forEachKey)
			entries[forEachKey] = value
		}
		sort.Strings(keys)
		ordered := make([]string, len(keys))
		for i, k := range keys {
			ordered[i] = entries[k]
		}
		out[base] = ordered
	}
	return out
}
