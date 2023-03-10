package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/graphic"
	gltfLoader "github.com/g3n/engine/loader/gltf"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/ngaut/log"
)

var bufferCache map[int][]byte
var bufferViewCache map[int][]byte

type GLTFLoader struct {
	*gltfLoader.GLTF
	path string
	data []byte
}

// ParseBinReader parses the glTF data from the specified binary reader
// and returns a pointer to the parsed structure
func ParseBinReader(r io.Reader, path string) (*GLTFLoader, error) {

	// Read header
	var header gltfLoader.GLBHeader
	err := binary.Read(r, binary.LittleEndian, &header)
	if err != nil {
		return nil, err
	}

	// Check magic and version
	if header.Magic != gltfLoader.GLBMagic {
		return nil, fmt.Errorf("invalid GLB Magic field")
	}
	if header.Version < 2 {
		return nil, fmt.Errorf("GLB version:%v not supported", header.Version)
	}

	// Read first chunk (JSON)
	buf, err := readChunk(r, gltfLoader.GLBJson)
	if err != nil {
		return nil, err
	}

	// Parse JSON into gltf object
	bb := bytes.NewBuffer(buf)
	gltf, err := gltfLoader.ParseJSONReader(bb, path)
	if err != nil {
		return nil, err
	}

	// Check for and read second chunk (binary, optional)
	data, err := readChunk(r, gltfLoader.GLBBin)
	if err != nil {
		return nil, err
	}

	return &GLTFLoader{GLTF: gltf, data: data}, nil
}

// readChunk reads a GLB chunk with the specified type and returns the data in a byte array.
func readChunk(r io.Reader, chunkType uint32) ([]byte, error) {

	// Read chunk header
	var chunk gltfLoader.GLBChunk
	err := binary.Read(r, binary.LittleEndian, &chunk)
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}

	// Check chunk type
	if chunk.Type != chunkType {
		return nil, fmt.Errorf("expected GLB chunk type [%v] but found [%v]", chunkType, chunk.Type)
	}

	// Read chunk data
	data := make([]byte, chunk.Length)
	err = binary.Read(r, binary.LittleEndian, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// ParseBin parses the glTF data from the specified binary file
// and returns a pointer to the parsed structure.
func ParseBin(filename string) (*GLTFLoader, error) {

	// Open file
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	// Extract path from file
	path := filepath.Dir(filename)
	defer f.Close()
	gltfDoc, err := gltfLoader.ParseBinReader(f, path)
	if err != nil {
		return nil, err
	}
	return &GLTFLoader{GLTF: gltfDoc, path: path}, nil
}

// validateAccessor validates the specified attribute accessor with the specified allowed types and component types.
func (g *GLTFLoader) validateAccessor(ac gltfLoader.Accessor, usage string, validTypes []string, validComponentTypes []int) error {

	// Validate accessor type
	validType := false
	for _, vType := range validTypes {
		if ac.Type == vType {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("invalid Accessor.Type %v for %s", ac.Type, usage)
	}

	// Validate accessor component type
	validComponentType := false
	for _, vComponentType := range validComponentTypes {
		if ac.ComponentType == vComponentType {
			validComponentType = true
			break
		}
	}
	if !validComponentType {
		return fmt.Errorf("invalid Accessor.ComponentType %v for %s", ac.ComponentType, usage)
	}

	return nil
}

// loadAccessorU32 loads data from the specified accessor and performs validation of the Type and ComponentType.
func (g *GLTFLoader) loadAccessorU32(ai int, usage string, validTypes []string, validComponentTypes []int) (math32.ArrayU32, error) {

	// Get Accessor for the specified index
	ac := g.Accessors[ai]

	// Validate type and component type
	err := g.validateAccessor(ac, usage, validTypes, validComponentTypes)
	if err != nil {
		return nil, err
	}

	// Load bytes
	data, err := g.loadAccessorBytes(ac)
	if err != nil {
		return nil, err
	}

	return g.bytesToArrayU32(data, ac.ComponentType, ac.Count*gltfLoader.TypeSizes[ac.Type])
}

// loadAccessorF32 loads data from the specified accessor and performs validation of the Type and ComponentType.
func (g *GLTFLoader) loadAccessorF32(ai int, usage string, validTypes []string, validComponentTypes []int) (math32.ArrayF32, error) {

	// Get Accessor for the specified index
	ac := g.Accessors[ai]

	// Validate type and component type
	err := g.validateAccessor(ac, usage, validTypes, validComponentTypes)
	if err != nil {
		return nil, err
	}

	// Load bytes
	data, err := g.loadAccessorBytes(ac)
	if err != nil {
		return nil, err
	}

	return g.bytesToArrayF32(data, ac.ComponentType, ac.Count*gltfLoader.TypeSizes[ac.Type])
}

// loadAccessorBytes returns the base byte array used by an accessor.
func (g *GLTFLoader) loadAccessorBytes(ac gltfLoader.Accessor) ([]byte, error) {

	// Get the Accessor's BufferView
	if ac.BufferView == nil && ac.Sparse == nil {
		return make([]byte, gltfLoader.TypeSizes[ac.Type]*ac.Count), nil
	}
	// if ac.BufferView == nil {
	// 	defaultIndex := 0
	// 	ac.BufferView = &defaultIndex
	// 	fmt.Println("loadAccessorBytes have not buffer view", ac.Sparse)
	// 	// return nil, fmt.Errorf("accessor.BufferView == nil NOT SUPPORTED YET") // TODO
	// }
	bv := g.BufferViews[*ac.BufferView]

	// Loads data from associated BufferView
	data, err := g.loadBufferView(*ac.BufferView)
	if err != nil {
		return nil, err
	}

	// Accessor offset into BufferView
	offset := 0
	if ac.ByteOffset != nil {
		offset = *ac.ByteOffset
	}
	data = data[offset:]

	// TODO check if interleaved and de-interleave if necessary?

	// Calculate the size in bytes of a complete attribute
	itemSize := gltfLoader.TypeSizes[ac.Type]
	itemBytes := int(gls.FloatSize) * itemSize

	// If the BufferView stride is equal to the item size, the buffer is not interleaved
	if (bv.ByteStride != nil) && (*bv.ByteStride != itemBytes) {
		// BufferView data is interleaved, de-interleave
		// TODO
		return nil, fmt.Errorf("data is interleaved - not supported for animation yet")
	}

	// TODO Sparse accessor

	return data, nil
}

// loadBufferView loads and returns a byte slice with data from the specified BufferView.
func (g *GLTFLoader) loadBufferView(bvIdx int) ([]byte, error) {

	// Check if provided buffer view index is valid
	if bvIdx < 0 || bvIdx >= len(g.BufferViews) {
		return nil, fmt.Errorf("invalid buffer view index")
	}
	bvData := g.BufferViews[bvIdx]
	// Return cached if available
	if cache, ex := bufferViewCache[bvIdx]; ex {
		return cache, nil
	}

	// Load buffer view buffer
	buf, err := g.loadBuffer(bvData.Buffer)
	if err != nil {
		return nil, err
	}

	// Establish offset
	offset := 0
	if bvData.ByteOffset != nil {
		offset = *bvData.ByteOffset
	}

	// Compute and return offset slice
	bvBytes := buf[offset : offset+bvData.ByteLength]

	// Cache buffer view
	bufferViewCache[bvIdx] = bvBytes

	return bvBytes, nil
}

// loadBuffer loads and returns the data from the specified GLTF Buffer index
func (g *GLTFLoader) loadBuffer(bufIdx int) ([]byte, error) {

	// Check if provided buffer index is valid
	if bufIdx < 0 || bufIdx >= len(g.Buffers) {
		return nil, fmt.Errorf("invalid buffer index")
	}
	bufData := &g.Buffers[bufIdx]
	// Return cached if available
	if cache, ex := bufferCache[bufIdx]; ex {
		return cache, nil
	}

	// If buffer URI use the chunk data field
	if bufData.Uri == "" {
		return g.data, nil
	}

	// Checks if buffer URI is a data URI
	var data []byte
	var err error
	if isDataURL(bufData.Uri) {
		data, err = loadDataURL(bufData.Uri)
	} else {
		// Try to load buffer from file
		data, err = g.loadFileBytes(bufData.Uri)
	}
	if err != nil {
		return nil, err
	}

	// Checks data length
	if len(data) != bufData.ByteLength {
		return nil, fmt.Errorf("buffer:%d read data length:%d expected:%d", bufIdx, len(data), bufData.ByteLength)
	}
	// Cache buffer data
	bufferCache[bufIdx] = data

	return data, nil
}

// isDataURL checks if the specified string has the prefix of data URL.
func isDataURL(url string) bool {

	if strings.HasPrefix(url, dataURLprefix) {
		return true
	}
	return false
}

// loadDataURL decodes the specified data URI string (base64).
func loadDataURL(url string) ([]byte, error) {

	var du dataURL
	err := parseDataURL(url, &du)
	if err != nil {
		return nil, err
	}

	// Checks for valid media type
	found := false
	for i := 0; i < len(validMediaTypes); i++ {
		if validMediaTypes[i] == du.MediaType {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("data URI media type:%s not supported", du.MediaType)
	}

	// Checks encoding
	if du.Encoding != "base64" {
		return nil, fmt.Errorf("data URI encoding:%s not supported", du.Encoding)
	}

	// Decodes data from BASE64
	data, err := base64.StdEncoding.DecodeString(du.Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// parseDataURL tries to parse the specified string as a data URL with the format:
// data:[<mediatype>][;base64],<data>
// and if successfull returns true and updates the specified pointer with the parsed fields.
func parseDataURL(url string, du *dataURL) error {

	// Check prefix
	if !isDataURL(url) {
		return fmt.Errorf("specified string is not a data URL")
	}

	// Separate header from data
	body := url[len(dataURLprefix):]
	parts := strings.Split(body, ",")
	if len(parts) != 2 {
		return fmt.Errorf("data URI contains more than one ','")
	}
	du.Data = parts[1]

	// Separate media type from optional encoding
	res := strings.Split(parts[0], ";")
	du.MediaType = res[0]
	if len(res) < 2 {
		return nil
	}
	if len(res) >= 2 {
		du.Encoding = res[1]
	}
	return nil
}

const (
	dataURLprefix = "data:"
	mimeBIN       = "application/octet-stream"
	mimePNG       = "image/png"
	mimeJPEG      = "image/jpeg"
)

// dataURL describes a decoded data url string.
type dataURL struct {
	MediaType string
	Encoding  string
	Data      string
}

var validMediaTypes = []string{mimeBIN, mimePNG, mimeJPEG}

// loadFileBytes loads the file with specified path as a byte array.
func (g *GLTFLoader) loadFileBytes(uri string) ([]byte, error) {

	fpath := filepath.Join(g.path, uri)
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// bytesToArrayU32 converts a byte array to ArrayU32.
func (g *GLTFLoader) bytesToArrayU32(data []byte, componentType, count int) (math32.ArrayU32, error) {

	// If component is UNSIGNED_INT nothing to do
	if componentType == gltfLoader.UNSIGNED_INT {
		arr := (*[1 << 30]uint32)(unsafe.Pointer(&data[0]))[:count]
		return math32.ArrayU32(arr), nil
	}

	// Converts UNSIGNED_SHORT or SHORT to UNSIGNED_INT
	if componentType == gltfLoader.UNSIGNED_SHORT || componentType == gltfLoader.SHORT {
		out := math32.NewArrayU32(count, count)
		for i := 0; i < count; i++ {
			out[i] = uint32(data[i*2]) + uint32(data[i*2+1])*256
		}
		return out, nil
	}

	// Converts UNSIGNED_BYTE or BYTE to UNSIGNED_INT
	if componentType == gltfLoader.UNSIGNED_BYTE || componentType == gltfLoader.BYTE {
		out := math32.NewArrayU32(count, count)
		for i := 0; i < count; i++ {
			out[i] = uint32(data[i])
		}
		return out, nil
	}

	return nil, fmt.Errorf("unsupported Accessor ComponentType:%v", componentType)
}

// bytesToArrayF32 converts a byte array to ArrayF32.
func (g *GLTFLoader) bytesToArrayF32(data []byte, componentType, count int) (math32.ArrayF32, error) {

	// If component is UNSIGNED_INT nothing to do
	if componentType == gltfLoader.UNSIGNED_INT {
		arr := (*[1 << 30]float32)(unsafe.Pointer(&data[0]))[:count]
		return math32.ArrayF32(arr), nil
	}

	// Converts UNSIGNED_SHORT or SHORT to UNSIGNED_INT
	if componentType == gltfLoader.UNSIGNED_SHORT || componentType == gltfLoader.SHORT {
		out := math32.NewArrayF32(count, count)
		for i := 0; i < count; i++ {
			out[i] = float32(data[i*2]) + float32(data[i*2+1])*256
		}
		return out, nil
	}

	// Converts UNSIGNED_BYTE or BYTE to UNSIGNED_INT
	if componentType == gltfLoader.UNSIGNED_BYTE || componentType == gltfLoader.BYTE {
		out := math32.NewArrayF32(count, count)
		for i := 0; i < count; i++ {
			out[i] = float32(data[i])
		}
		return out, nil
	}

	return (*[1 << 30]float32)(unsafe.Pointer(&data[0]))[:count], nil
}

// loadAttributes loads the provided list of vertex attributes as VBO(s) into the specified geometry.
func (g *GLTF) loadAttributes(geom *geometry.Geometry, attributes map[string]int, indices math32.ArrayU32) error {

	// Indices of buffer views
	interleavedVBOs := make(map[int]*gls.VBO, 0)

	// Load primitive attributes
	for name, aci := range attributes {
		accessor := g.Accessors[aci]

		// Validate that accessor is compatible with attribute
		err := g.validateAccessorAttribute(accessor, name)
		if err != nil {
			return err
		}

		// Load data and add it to geometry's VBO
		if g.isInterleaved(accessor) {
			bvIdx := *accessor.BufferView
			// Check if we already loaded this buffer view
			vbo, ok := interleavedVBOs[bvIdx]
			if ok {
				// Already created VBO for this buffer view
				// Add attribute with correct byteOffset
				g.addAttributeToVBO(vbo, name, uint32(*accessor.ByteOffset))
			} else {
				// Load data and create vbo
				buf, err := g.loadBufferView(bvIdx)
				if err != nil {
					return err
				}
				//
				// TODO: BUG HERE
				// If buffer view has accessors with different component type then this will have a read alignment problem!
				//
				data, err := g.bytesToArrayF32(buf, accessor.ComponentType, accessor.Count*TypeSizes[accessor.Type])
				if err != nil {
					return err
				}
				vbo := gls.NewVBO(data)
				g.addAttributeToVBO(vbo, name, 0)
				// Save reference to VBO keyed by index of the buffer view
				interleavedVBOs[bvIdx] = vbo
				// Add VBO to geometry
				geom.AddVBO(vbo)
			}
		} else {
			buf, err := g.loadAccessorBytes(accessor)
			if err != nil {
				return err
			}
			data, err := g.bytesToArrayF32(buf, accessor.ComponentType, accessor.Count*TypeSizes[accessor.Type])
			if err != nil {
				return err
			}
			vbo := gls.NewVBO(data)
			g.addAttributeToVBO(vbo, name, 0)
			// Add VBO to geometry
			geom.AddVBO(vbo)
		}
	}

	// Set indices
	if len(indices) > 0 {
		geom.SetIndices(indices)
	}

	return nil
}

// LoadMesh creates and returns a Graphic Node (graphic.Mesh, graphic.Lines, graphic.Points, etc)
// from the specified GLTF.Meshes index.
func (g *GLTF) LoadMesh(meshIdx int) (core.INode, error) {

	// Check if provided mesh index is valid
	if meshIdx < 0 || meshIdx >= len(g.Meshes) {
		return nil, fmt.Errorf("invalid mesh index")
	}
	meshData := g.Meshes[meshIdx]
	// Return cached if available
	if meshData.cache != nil {
		// TODO CLONE/REINSTANCE INSTEAD
		//log.Debug("Instancing Mesh %d (from cached)", meshIdx)
		//return meshData.cache, nil
	}
	log.Debug("Loading Mesh %d", meshIdx)

	var err error

	// Create container node
	var meshNode core.INode
	meshNode = core.NewNode()

	for i := 0; i < len(meshData.Primitives); i++ {

		// Get primitive information
		p := meshData.Primitives[i]

		// Indexed Geometry
		indices := math32.NewArrayU32(0, 0)
		if p.Indices != nil {
			pidx, err := g.loadIndices(*p.Indices)
			if err != nil {
				return nil, err
			}
			indices = append(indices, pidx...)
		} else {
			// Non-indexed primitive
			// indices array stay empty
		}

		// Load primitive material
		var grMat material.IMaterial
		if p.Material != nil {
			grMat, err = g.LoadMaterial(*p.Material)
			if err != nil {
				return nil, err
			}
		} else {
			grMat = g.newDefaultMaterial()
		}

		// Create geometry
		var igeom geometry.IGeometry
		igeom = geometry.NewGeometry()
		geom := igeom.GetGeometry()

		err = g.loadAttributes(geom, p.Attributes, indices)
		if err != nil {
			return nil, err
		}

		// If primitive has targets then the geometry should be a morph geometry
		if len(p.Targets) > 0 {
			morphGeom := geometry.NewMorphGeometry(geom)

			// TODO Load morph target names if present in extras under "targetNames"
			// TODO Update morph target weights if present in Mesh.Weights

			// Load targets
			for i := range p.Targets {
				tGeom := geometry.NewGeometry()
				attributes := p.Targets[i]
				err = g.loadAttributes(tGeom, attributes, indices)
				if err != nil {
					return nil, err
				}
				morphGeom.AddMorphTargetDeltas(tGeom)
			}

			igeom = morphGeom
		}

		// Default mode is 4 (TRIANGLES)
		mode := TRIANGLES
		if p.Mode != nil {
			mode = *p.Mode
		}

		// Create Mesh
		// TODO materials for LINES, etc need to be different...
		if mode == TRIANGLES {
			meshNode.GetNode().Add(graphic.NewMesh(igeom, grMat))
		} else if mode == LINES {
			meshNode.GetNode().Add(graphic.NewLines(igeom, grMat))
		} else if mode == LINE_STRIP {
			meshNode.GetNode().Add(graphic.NewLineStrip(igeom, grMat))
		} else if mode == POINTS {
			meshNode.GetNode().Add(graphic.NewPoints(igeom, grMat))
		} else {
			return nil, fmt.Errorf("unsupported primitive:%v", mode)
		}
	}

	children := meshNode.GetNode().Children()
	if len(children) == 1 {
		meshNode = children[0]
	}

	// Cache mesh
	g.Meshes[meshIdx].cache = meshNode

	return meshNode, nil
}
