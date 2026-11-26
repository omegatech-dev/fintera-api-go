package services

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder for image.Decode
	_ "image/png"  // register PNG decoder for image.Decode

	"github.com/disintegration/imaging"
	webp "github.com/mayahiro/go-webp"
	_ "golang.org/x/image/webp" // register WebP decoder for image.Decode
)

// Image optimization limits and quality settings. Raster uploads are decoded,
// downscaled to the corresponding maximum dimension and re-encoded as lossy
// WebP to reduce storage and bandwidth. SVG and PDF uploads are never handled
// here: they are documents/vector assets and are stored unchanged.
const (
	// PhotoMaxDimension caps the long edge of stored gallery photos.
	PhotoMaxDimension = 2048
	// PhotoThumbnailDimension caps the long edge of gallery thumbnails.
	PhotoThumbnailDimension = 480
	// DrawingMaxDimension caps the long edge of raster plans.
	DrawingMaxDimension = 3000
	// ProfileMaxDimension caps the long edge of stored profile pictures.
	ProfileMaxDimension = 1024
	// ProfileThumbnailDimension is the square-ish avatar thumbnail size.
	ProfileThumbnailDimension = 128
	// BrandingLogoMaxDimension caps the long edge of branding logos.
	BrandingLogoMaxDimension = 1024
	// ReceiptMaxDimension caps the long edge of image receipts.
	ReceiptMaxDimension = 2048
	// IdentityDocumentMaxDimension caps the long edge of the identity
	// document previews.
	IdentityDocumentMaxDimension = 1600

	photoWebPQuality        = 80
	drawingWebPQuality      = 90
	profileWebPQuality      = 85
	brandingLogoWebPQuality = 90
	receiptWebPQuality      = 80
	identityDocumentQuality = 85
)

// ErrUnsupportedRaster is returned when an uploaded image cannot be decoded as
// a raster image (PNG/JPEG/WebP).
var ErrUnsupportedRaster = errors.New("la imagen no se pudo decodificar")

// ResizeToWebP decodes a raster image, downscales it so its longest edge is at
// most maxDimension and encodes it as lossy WebP. Images already within the
// limit are encoded without resizing.
func ResizeToWebP(data []byte, maxDimension, quality int) ([]byte, error) {
	img, err := decodeRaster(data)
	if err != nil {
		return nil, err
	}
	return encodeWebP(scaleToMax(img, maxDimension), quality)
}

// ResizeToWebPSet produces two WebP variants (full and thumbnail) from the same
// source bytes, decoding the source only once.
func ResizeToWebPSet(data []byte, maxDimension, thumbDimension, quality int) (full, thumb []byte, err error) {
	img, err := decodeRaster(data)
	if err != nil {
		return nil, nil, err
	}
	full, err = encodeWebP(scaleToMax(img, maxDimension), quality)
	if err != nil {
		return nil, nil, err
	}
	thumb, err = encodeWebP(scaleToMax(img, thumbDimension), quality)
	if err != nil {
		return nil, nil, err
	}
	return full, thumb, nil
}

// PhotoToWebP returns the stored gallery photo variant and its thumbnail.
func PhotoToWebP(data []byte) (full, thumb []byte, err error) {
	return ResizeToWebPSet(data, PhotoMaxDimension, PhotoThumbnailDimension, photoWebPQuality)
}

// DrawingToWebP returns a raster plan encoded as WebP.
func DrawingToWebP(data []byte) ([]byte, error) {
	return ResizeToWebP(data, DrawingMaxDimension, drawingWebPQuality)
}

// ProfileToWebP returns the stored profile picture variant and its thumbnail.
func ProfileToWebP(data []byte) (full, thumb []byte, err error) {
	return ResizeToWebPSet(data, ProfileMaxDimension, ProfileThumbnailDimension, profileWebPQuality)
}

// LogoToWebP returns a branding logo encoded as WebP (alpha preserved).
func LogoToWebP(data []byte) ([]byte, error) {
	return ResizeToWebP(data, BrandingLogoMaxDimension, brandingLogoWebPQuality)
}

// ReceiptToWebP returns an image receipt encoded as WebP.
func ReceiptToWebP(data []byte) ([]byte, error) {
	return ResizeToWebP(data, ReceiptMaxDimension, receiptWebPQuality)
}

// IdentityDocumentToWebP returns the preview variant of an identity
// document. The formats the Go decoders cannot read (PDF, HEIC, HEIF) return
// an error, and the caller stores the document without a preview instead of
// rejecting it.
func IdentityDocumentToWebP(data []byte) ([]byte, error) {
	return ResizeToWebP(data, IdentityDocumentMaxDimension, identityDocumentQuality)
}

func decodeRaster(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedRaster, err)
	}
	return img, nil
}

func scaleToMax(img image.Image, maxDimension int) image.Image {
	if maxDimension <= 0 {
		return img
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= maxDimension && height <= maxDimension {
		return img
	}
	if width >= height {
		return imaging.Resize(img, maxDimension, 0, imaging.Lanczos)
	}
	return imaging.Resize(img, 0, maxDimension, imaging.Lanczos)
}

func encodeWebP(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	opts := &webp.Options{
		Compression: webp.CompressionLossy,
		Mode:        webp.ModeLossyQuality,
		Quality:     quality,
	}
	if err := webp.Encode(&buf, img, opts); err != nil {
		return nil, fmt.Errorf("error al generar WebP: %w", err)
	}
	return buf.Bytes(), nil
}
