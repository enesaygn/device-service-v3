// 📁 internal/discovery/usb/database.go - USB Device Database
package usb

import (
	"device-service/internal/model"

	"github.com/google/gousb"
)

// DeviceDatabase contains known USB devices for identification
type DeviceDatabase struct {
	vendors map[gousb.ID]*VendorInfo
}

// VendorInfo contains vendor-specific information
type VendorInfo struct {
	Brand    model.DeviceBrand
	Name     string
	products map[gousb.ID]*ProductInfo
}

// ProductInfo contains product-specific information
type ProductInfo struct {
	Model        string
	DeviceType   model.DeviceType
	Capabilities []string
	Confidence   float64
}

// NewDeviceDatabase creates and initializes the device database
func NewDeviceDatabase() *DeviceDatabase {
	db := &DeviceDatabase{
		vendors: make(map[gousb.ID]*VendorInfo),
	}
	db.initializeDatabase()
	return db
}

// initializeDatabase populates the known devices database
func (db *DeviceDatabase) initializeDatabase() {
	// EPSON vendor (0x04B8)
	epsonVendor := &VendorInfo{
		Brand:    model.BrandEpson,
		Name:     "Seiko Epson Corporation",
		products: make(map[gousb.ID]*ProductInfo),
	}

	// EPSON products
	epsonVendor.products[0x0202] = &ProductInfo{
		Model:        "TM-T88IV",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER"},
		Confidence:   0.95,
	}
	epsonVendor.products[0x0203] = &ProductInfo{
		Model:        "TM-T88V",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER", "BEEP"},
		Confidence:   0.95,
	}
	epsonVendor.products[0x0214] = &ProductInfo{
		Model:        "TM-T88VI",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER", "LOGO", "BEEP"},
		Confidence:   0.95,
	}
	epsonVendor.products[0x0215] = &ProductInfo{
		Model:        "TM-T20III",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT"},
		Confidence:   0.90,
	}
	epsonVendor.products[0x0216] = &ProductInfo{
		Model:        "TM-T82III",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER"},
		Confidence:   0.90,
	}
	epsonVendor.products[0x0217] = &ProductInfo{
		Model:        "TM-M30",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER", "BLUETOOTH"},
		Confidence:   0.95,
	}

	db.vendors[0x04B8] = epsonVendor

	// STAR vendor (0x0519)
	starVendor := &VendorInfo{
		Brand:    model.BrandStar,
		Name:     "Star Micronics Co., Ltd.",
		products: make(map[gousb.ID]*ProductInfo),
	}

	starVendor.products[0x0001] = &ProductInfo{
		Model:        "TSP143III",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT"},
		Confidence:   0.90,
	}
	starVendor.products[0x0002] = &ProductInfo{
		Model:        "TSP143IIIU",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT"},
		Confidence:   0.90,
	}
	starVendor.products[0x0003] = &ProductInfo{
		Model:        "TSP654II",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER"},
		Confidence:   0.85,
	}

	db.vendors[0x0519] = starVendor

	// CITIZEN vendor (0x1CBE)
	citizenVendor := &VendorInfo{
		Brand:    model.BrandCitizen,
		Name:     "Citizen Systems Japan Co., Ltd.",
		products: make(map[gousb.ID]*ProductInfo),
	}

	citizenVendor.products[0x0001] = &ProductInfo{
		Model:        "CT-S310II",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT"},
		Confidence:   0.85,
	}
	citizenVendor.products[0x0002] = &ProductInfo{
		Model:        "CT-S4000",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER"},
		Confidence:   0.85,
	}

	db.vendors[0x1CBE] = citizenVendor

	// BIXOLON vendor (0x1504)
	bixolonVendor := &VendorInfo{
		Brand:    model.BrandBixolon,
		Name:     "BIXOLON Co., Ltd.",
		products: make(map[gousb.ID]*ProductInfo),
	}

	bixolonVendor.products[0x0006] = &ProductInfo{
		Model:        "SRP-330II",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT"},
		Confidence:   0.80,
	}
	bixolonVendor.products[0x0007] = &ProductInfo{
		Model:        "SRP-350III",
		DeviceType:   model.DeviceTypePrinter,
		Capabilities: []string{"PRINT", "CUT", "DRAWER"},
		Confidence:   0.80,
	}

	db.vendors[0x1504] = bixolonVendor
}

// IsKnownVendor checks if a vendor ID is in the database
func (db *DeviceDatabase) IsKnownVendor(vendorID gousb.ID) bool {
	_, exists := db.vendors[vendorID]
	return exists
}

// GetVendorInfo retrieves vendor information
func (db *DeviceDatabase) GetVendorInfo(vendorID gousb.ID) *VendorInfo {
	return db.vendors[vendorID]
}

// GetProductInfo retrieves product information from vendor
func (vi *VendorInfo) GetProductInfo(productID gousb.ID) *ProductInfo {
	return vi.products[productID]
}

// GetTotalProductCount returns total number of known products
func (db *DeviceDatabase) GetTotalProductCount() int {
	total := 0
	for _, vendor := range db.vendors {
		total += len(vendor.products)
	}
	return total
}

// AddVendor adds a new vendor to the database
func (db *DeviceDatabase) AddVendor(vendorID gousb.ID, info *VendorInfo) {
	if info.products == nil {
		info.products = make(map[gousb.ID]*ProductInfo)
	}
	db.vendors[vendorID] = info
}

// AddProduct adds a new product to an existing vendor
func (db *DeviceDatabase) AddProduct(vendorID, productID gousb.ID, info *ProductInfo) {
	if vendor, exists := db.vendors[vendorID]; exists {
		vendor.products[productID] = info
	}
}
