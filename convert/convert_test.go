package convert

import (
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverter_ConvertV3ToV31_VersionCheck(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid 3.0.0 version",
			input: `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0`,
			wantErr: false,
		},
		{
			name: "valid 3.0.1 version",
			input: `openapi: 3.0.1
info:
  title: Test API
  version: 1.0.0`,
			wantErr: false,
		},
		{
			name: "invalid 2.0 version",
			input: `swagger: "2.0"
info:
  title: Test API
  version: 1.0.0`,
			wantErr:     true,
			errContains: "is not OpenAPI 3.0.x",
		},
		{
			name: "invalid 3.1.0 version",
			input: `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0`,
			wantErr:     true,
			errContains: "is not OpenAPI 3.0.x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := libopenapi.NewDocument([]byte(tt.input))
			require.NoError(t, err)

			converter := NewConverter(&doc)
			result, err := converter.ConvertV3ToV31()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, "3.1.0", (*result).GetVersion())
			}
		})
	}
}

func TestConverter_ConvertV3ToV31_ExampleConversion(t *testing.T) {
	input := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: string
      example: "example value"`

	doc, err := libopenapi.NewDocument([]byte(input))
	require.NoError(t, err)

	converter := NewConverter(&doc)
	result, err := converter.ConvertV3ToV31()
	require.NoError(t, err)

	model, errs := (*result).BuildV3Model()
	require.Empty(t, errs)

	// check simply that examples is set with the correct value
	testSchema, ok := model.Model.Components.Schemas.Get("Test")
	require.True(t, ok)
	require.NotNil(t, testSchema)
	assert.NotNil(t, testSchema.Schema().Examples)
	assert.Equal(t, 1, len(testSchema.Schema().Examples))
	assert.Equal(t, "example value", testSchema.Schema().Examples[0].Value)
	assert.Nil(t, testSchema.Schema().Example)
}

func TestConverter_ConvertV3ToV31_FileUploadPayload(t *testing.T) {
	input := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /upload-octet:
    post:
      requestBody:
        content:
          application/octet-stream:
            schema:
              type: string
              format: binary
  /update-octet:
    put:
      requestBody:
        content:
          application/octet-stream:
            schema:
              type: string
              format: binary
  /upload-png:
    post:
      requestBody:
        content:
          image/png:
            schema:
              type: string
              format: binary
  /update-png:
    put:
      requestBody:
        content:
          image/png:
            schema:
              type: string
              format: binary
  /upload-base64:
    post:
      requestBody:
        content:
          application/octet-stream:
            schema:
              type: string
              format: base64
  /update-base64:
    put:
      requestBody:
        content:
          application/octet-stream:
            schema:
              type: string
              format: base64
  /upload-multipart:
    post:
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                orderId:
                  type: integer
                binaryFile:
                  type: string
                  format: binary
                base64File:
                  type: string
                  format: base64
                normalField:
                  type: string
  /download-binary:
    get:
      responses:
        200:
          content:
            application/octet-stream:
              schema:
                type: string
                format: binary
  /put-binary:
    put:
      requestBody:
        content:
          application/octet-stream:
            schema:
              type: string
              format: binary`

	doc, err := libopenapi.NewDocument([]byte(input))
	require.NoError(t, err)

	converter := NewConverter(&doc)
	result, err := converter.ConvertV3ToV31()
	require.NoError(t, err)

	model, errs := (*result).BuildV3Model()
	require.Empty(t, errs)

	// Check POST binary upload (should remove schema)
	uploadPath, ok := model.Model.Paths.PathItems.Get("/upload-octet")
	require.True(t, ok)
	post := uploadPath.Post
	require.NotNil(t, post)
	mediaType, ok := post.RequestBody.Content.Get("application/octet-stream")
	require.True(t, ok)
	assert.Nil(t, mediaType.Schema, "Schema should be removed for POST binary uploads")

	// Check PUT binary upload (should also remove schema)
	updatePath, ok := model.Model.Paths.PathItems.Get("/update-octet")
	require.True(t, ok)
	put := updatePath.Put
	require.NotNil(t, put)
	mediaType, ok = put.RequestBody.Content.Get("application/octet-stream")
	require.True(t, ok)
	assert.Nil(t, mediaType.Schema, "Schema should be removed for PUT binary uploads")

	// Check POST PNG binary upload (should remove schema)
	pngPath, ok := model.Model.Paths.PathItems.Get("/upload-png")
	require.True(t, ok)
	post = pngPath.Post
	require.NotNil(t, post)
	mediaType, ok = post.RequestBody.Content.Get("image/png")
	require.True(t, ok)
	assert.Nil(t, mediaType.Schema, "Schema should be removed for POST PNG binary uploads")

	// Check PUT PNG binary upload (should remove schema)
	pngUpdatePath, ok := model.Model.Paths.PathItems.Get("/update-png")
	require.True(t, ok)
	put = pngUpdatePath.Put
	require.NotNil(t, put)
	mediaType, ok = put.RequestBody.Content.Get("image/png")
	require.True(t, ok)
	assert.Nil(t, mediaType.Schema, "Schema should be removed for PUT PNG binary uploads")

	// Check POST base64 upload (should keep schema and set contentEncoding)
	base64Path, ok := model.Model.Paths.PathItems.Get("/upload-base64")
	require.True(t, ok)
	post = base64Path.Post
	require.NotNil(t, post)
	mediaType, ok = post.RequestBody.Content.Get("application/octet-stream")
	require.True(t, ok)
	assert.NotNil(t, mediaType.Schema, "Schema should not be removed for POST base64 uploads")
	assert.Empty(t, mediaType.Schema.Schema().Format)
	assert.Equal(t, "base64", mediaType.Schema.Schema().ContentEncoding)

	// Check PUT base64 upload (should keep schema and set contentEncoding)
	base64UpdatePath, ok := model.Model.Paths.PathItems.Get("/update-base64")
	require.True(t, ok)
	put = base64UpdatePath.Put
	require.NotNil(t, put)
	mediaType, ok = put.RequestBody.Content.Get("application/octet-stream")
	require.True(t, ok)
	assert.NotNil(t, mediaType.Schema, "Schema should not be removed for PUT base64 uploads")
	assert.Equal(t, "base64", mediaType.Schema.Schema().ContentEncoding)

	// Check multipart form data (properties should be converted appropriately)
	multipartPath, ok := model.Model.Paths.PathItems.Get("/upload-multipart")
	require.True(t, ok)
	post = multipartPath.Post
	require.NotNil(t, post)
	mediaType, ok = post.RequestBody.Content.Get("multipart/form-data")
	require.True(t, ok)
	require.NotNil(t, mediaType.Schema)
	schema := mediaType.Schema.Schema()
	orderIdProp, ok := schema.Properties.Get("orderId")
	require.True(t, ok)
	assert.Equal(t, []string{"integer"}, orderIdProp.Schema().Type)
	binaryFileProp, ok := schema.Properties.Get("binaryFile")
	require.True(t, ok)
	assert.Equal(t, "application/octet-stream", binaryFileProp.Schema().ContentMediaType, "Binary file property should use contentMediaType")
	assert.Empty(t, binaryFileProp.Schema().Format)
	base64FileProp, ok := schema.Properties.Get("base64File")
	require.True(t, ok)
	assert.Equal(t, "base64", base64FileProp.Schema().ContentEncoding, "Base64 file property should use contentEncoding")
	assert.Empty(t, base64FileProp.Schema().Format)
	normalProp, ok := schema.Properties.Get("normalField")
	require.True(t, ok)
	assert.Equal(t, []string{"string"}, normalProp.Schema().Type)
	assert.Empty(t, normalProp.Schema().Format)

	// Check GET response (should keep schema)
	downloadPath, ok := model.Model.Paths.PathItems.Get("/download-binary")
	require.True(t, ok)
	get := downloadPath.Get
	require.NotNil(t, get)
	response, ok := get.Responses.Codes.Get("200")
	require.True(t, ok)
	mediaType, ok = response.Content.Get("application/octet-stream")
	require.True(t, ok)
	// assert.NotNil(t, mediaType.Schema, "Schema should not be removed for GET responses")

	// Check PUT request (should keep schema)
	// putPath, ok := model.Model.Paths.PathItems.Get("/put-binary")
	// require.True(t, ok)
	// put := putPath.Put
	// require.NotNil(t, put)
	// mediaType, ok = put.RequestBody.Content.Get("application/octet-stream")
	// require.True(t, ok)
	// assert.NotNil(t, mediaType.Schema, "Schema should not be removed for PUT requests")
	// fmt.Println("md ce", mediaType.Schema.Schema().GoLow().ContentEncoding.Value)

}

func TestConverter_ConvertV3ToV31_SchemaConversion(t *testing.T) {
	input := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
components:
  schemas:
    Test:
      type: object
      nullable: true
      properties:
        field1:
          type: string
          nullable: true
        field2:
          type: array
          nullable: false # can also leave empty
          items:
            type: integer
            nullable: true
        field3:
          exclusiveMinimum: true
          minimum: 10`

	doc, err := libopenapi.NewDocument([]byte(input))
	require.NoError(t, err)

	converter := NewConverter(&doc)
	result, err := converter.ConvertV3ToV31()
	require.NoError(t, err)

	// Build the model to check the converted schemas
	model, errs := (*result).BuildV3Model()
	require.Empty(t, errs)
	require.NotNil(t, model)

	// Verify conversion results
	testSchemaProxy, ok := model.Model.Components.Schemas.Get("Test")
	require.True(t, ok)
	require.NotNil(t, testSchemaProxy)

	testSchema, err := testSchemaProxy.BuildSchema()
	require.NoError(t, err)
	assert.NotNil(t, testSchema)

	// Check that nullable has been converted to type array
	assert.Equal(t, []string{"object", "null"}, testSchema.Type)
	assert.Nil(t, testSchema.Nullable)

	// Check field1
	field1Proxy, ok := testSchema.Properties.Get("field1")
	require.True(t, ok)
	require.NotNil(t, field1Proxy)
	field1 := field1Proxy.Schema()
	assert.Equal(t, []string{"string", "null"}, field1.Type)
	assert.Nil(t, field1.Nullable)

	// // Check field2
	field2Proxy, ok := testSchema.Properties.Get("field2")
	require.True(t, ok)
	require.NotNil(t, field2Proxy)
	field2 := field2Proxy.Schema()
	assert.Equal(t, []string{"array"}, field2.Type)
	assert.Nil(t, field2.Nullable)
	require.True(t, field2.Items.IsA())
	assert.Equal(t, []string{"integer", "null"}, field2.Items.A.Schema().Type)

	// Check field3
	field3Proxy, ok := testSchema.Properties.Get("field3")
	require.True(t, ok)
	require.NotNil(t, field3Proxy)
	field3 := field3Proxy.Schema()
	// assert.Equal(t, []string{}, field3.Type) `actual: []string(nil)`
	assert.Nil(t, field3.Nullable)
	require.True(t, field3.ExclusiveMinimum.IsB())
	assert.Equal(t, 10.0, field3.ExclusiveMinimum.B)
}
