package parts

import (
	"crypto/sha256"
	"fmt"
	"path"
	"strings"

	"github.com/theAstiv/docx-go/internal/opc"
)

// PathMediaDir is the directory Word stores embedded images in.
const PathMediaDir = "word/media"

// imageContentTypes maps the extensions AddImage accepts to MIME types.
var imageContentTypes = map[string]string{
	"png":  "image/png",
	"jpeg": "image/jpeg",
	"jpg":  "image/jpeg",
	"gif":  "image/gif",
}

// ImageContentType returns the MIME type for an image extension (without the
// dot), or "" when the extension is not supported.
func ImageContentType(ext string) string {
	return imageContentTypes[strings.ToLower(ext)]
}

// loadImageIndex builds the hash index of media parts already in the package.
func (p *Package) loadImageIndex() {
	if p.imgByHash != nil {
		return
	}
	p.imgByHash = make(map[[sha256.Size]byte]string)
	for _, name := range p.pkg.PartPaths() {
		if path.Dir(name) == PathMediaDir {
			sum := sha256.Sum256(p.pkg.Part(name))
			if _, dup := p.imgByHash[sum]; !dup {
				p.imgByHash[sum] = name
			}
		}
	}
}

// nextMediaName returns an unused word/media/imageN.ext path.
func (p *Package) nextMediaName(ext string) string {
	n := 1
	for _, name := range p.pkg.PartPaths() {
		if path.Dir(name) != PathMediaDir {
			continue
		}
		var i int
		base := strings.TrimSuffix(path.Base(name), path.Ext(name))
		if _, err := fmt.Sscanf(base, "image%d", &i); err == nil && i >= n {
			n = i + 1
		}
	}
	return fmt.Sprintf("%s/image%d.%s", PathMediaDir, n, ext)
}

// AddImage stores data as a media part and returns the ID of an image
// relationship from the part at source to it, along with the media file name.
//
// Identical bytes are stored once: a second call with the same data reuses
// the existing part, and reuses the relationship too when source already has
// one to it. ext is the lower-case extension without a dot; it must be one
// ImageContentType knows.
func (p *Package) AddImage(source string, data []byte, ext string) (relID, fileName string, err error) {
	ext = strings.ToLower(ext)
	if ImageContentType(ext) == "" {
		return "", "", fmt.Errorf("parts: add image: unsupported extension %q", ext)
	}
	p.loadImageIndex()

	sum := sha256.Sum256(data)
	mediaPath, ok := p.imgByHash[sum]
	if !ok {
		mediaPath = p.nextMediaName(ext)
		p.pkg.SetPart(mediaPath, append([]byte(nil), data...))
		p.imgByHash[sum] = mediaPath
	}
	// A reused part may carry a different extension than ext (jpg vs jpeg).
	mediaExt := strings.TrimPrefix(path.Ext(mediaPath), ".")
	if mediaCT := ImageContentType(mediaExt); mediaCT != "" {
		p.pkg.ContentTypes.SetDefault(mediaExt, mediaCT)
	}

	target, relErr := relativeTarget(source, mediaPath)
	if relErr != nil {
		return "", "", relErr
	}
	rels := p.pkg.Rels(source)
	for _, r := range rels.ByType(opc.RelTypeImage) {
		if r.TargetMode == "" && r.Target == target {
			return r.ID, path.Base(mediaPath), nil
		}
	}
	return rels.Add(opc.RelTypeImage, target, ""), path.Base(mediaPath), nil
}

// relativeTarget returns the relationship target that reaches partPath from
// the directory of source.
func relativeTarget(source, partPath string) (string, error) {
	dir := path.Dir(source)
	if !strings.HasPrefix(partPath, dir+"/") {
		return "", fmt.Errorf("parts: image %q is not below %q", partPath, dir)
	}
	return strings.TrimPrefix(partPath, dir+"/"), nil
}

// ImageData returns the bytes of the image that the relationship relID of the
// part at source points to. It reports false when the relationship is missing,
// is not an internal image relationship, or its part is absent.
func (p *Package) ImageData(source, relID string) ([]byte, bool) {
	r, ok := p.pkg.Rels(source).ByID(relID)
	if !ok || r.Type != opc.RelTypeImage || r.TargetMode == targetModeExternal {
		return nil, false
	}
	data := p.pkg.Part(resolveRelPath(source, r.Target))
	if data == nil {
		return nil, false
	}
	return data, true
}
