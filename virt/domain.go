package virt

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
)

const (
	vmmMetaTag       = "vmm"
	vmmMetaNamespace = "http://calebstew.art/xmlns/vmm"
)

var (
	vmmDefaultMetadata = VmmDomainMetadata{
		Path:   "/",
		Labels: []string{},
	}
)

type VmmDomainMetadata struct {
	Path    string   `xml:"path"`
	Labels  []string `xml:"label"`
	XMLName xml.Name `xml:"vmm"`
}

type Domain struct {
	libvirt.Domain // Core domain conneciton
}

func NewDomain(domain libvirt.Domain) (*Domain, error) {
	dom := Domain{
		Domain: domain,
	}

	return &dom, nil
}

func (dom *Domain) GetVmmData() VmmDomainMetadata {
	metadata := VmmDomainMetadata{
		Path:   "/",
		Labels: []string{},
	}

	if xmlData, err := dom.GetMetadata(libvirt.DOMAIN_METADATA_ELEMENT, vmmMetaNamespace, libvirt.DOMAIN_AFFECT_CURRENT); err != nil {
		return vmmDefaultMetadata
	} else if err := xml.Unmarshal([]byte(xmlData), &metadata); err != nil {
		return vmmDefaultMetadata
	} else {
		if !strings.HasSuffix(metadata.Path, "/") {
			metadata.Path = metadata.Path + "/"
		}
		return metadata
	}
}

func (dom *Domain) UpdateVmmData(metadata VmmDomainMetadata) error {
	if xmlData, err := xml.Marshal(metadata); err != nil {
		return err
	} else {
		return dom.SetMetadata(
			libvirt.DOMAIN_METADATA_ELEMENT,
			string(xmlData),
			vmmMetaTag,
			vmmMetaNamespace,
			libvirt.DOMAIN_AFFECT_CONFIG,
		)
	}
}

func (dom *Domain) SaveState() error {
	return nil
}

func (dom *Domain) RestoreSavedState() error {
	return nil
}

func (dom *Domain) Clone(virt *Connection, name string, linked bool) (newDomain *Domain, err error) {

	description := libvirtxml.Domain{}
	if xmlDesc, err := dom.GetXMLDesc(libvirt.DOMAIN_XML_SECURE); err != nil {
		return nil, err
	} else if err := xml.Unmarshal([]byte(xmlDesc), &description); err != nil {
		return nil, err
	}

	description.Name = name
	description.UUID = uuid.NewString()

	createdVolumes := []*libvirt.StorageVol{}

	for idx, disk := range description.Devices.Disks {
		// Ignore read-only disks
		if disk.ReadOnly != nil {
			continue
		}

		// Clone the disk
		vol, err := dom.cloneDisk(virt, &description, idx, linked)
		if err != nil {
			dom.cleanupVolumes(virt, createdVolumes)
			return nil, err
		} else if vol != nil {
			createdVolumes = append(createdVolumes, vol)
		}
	}

	if xmlDesc, err := xml.Marshal(&description); err != nil {
		dom.cleanupVolumes(virt, createdVolumes)
		return nil, err
	} else if libvirtDomain, err := virt.DomainDefineXML(string(xmlDesc)); err != nil {
		dom.cleanupVolumes(virt, createdVolumes)
		return nil, err
	} else {
		return NewDomain(*libvirtDomain)
	}
}

func (dom *Domain) cleanupVolumes(virt *Connection, volumes []*libvirt.StorageVol) {
	for _, volume := range volumes {
		volume.Delete(libvirt.STORAGE_VOL_DELETE_NORMAL)
	}
}

// Clone the disk at the specified disk index
func (dom *Domain) cloneDisk(virt *Connection, description *libvirtxml.Domain, idx int, linked bool) (*libvirt.StorageVol, error) {
	disk := &description.Devices.Disks[idx]

	// Nothing to clone, but no error
	if disk.Source == nil || disk.Source.File == nil {
		return nil, nil
	}

	// Grab the backing volume from the disk source path
	backingVolume, err := virt.LookupStorageVolByPath(disk.Source.File.File)
	if err != nil {
		return nil, err
	}

	// Lookup the pool the volume resides in (we always clone to the same pool)
	backingPool, err := backingVolume.LookupPoolByVolume()
	if err != nil {
		return nil, err
	}

	// Load the backing volume XML description
	volumeDescription := libvirtxml.StorageVolume{}
	if xmlDesc, err := backingVolume.GetXMLDesc(0); err != nil {
		return nil, err
	} else if err := xml.Unmarshal([]byte(xmlDesc), &volumeDescription); err != nil {
		return nil, err
	}

	// Grab the backing volume target format if available
	volumeTargetFormat := ""
	if volumeDescription.Target != nil && volumeDescription.Target.Format != nil {
		volumeTargetFormat = volumeDescription.Target.Format.Type
	}

	// Create a new volume description which matches the backing volume
	newVolumeDescription := libvirtxml.StorageVolume{
		Type: volumeDescription.Type,
		Name: fmt.Sprintf("%v.qcow2", description.Name),
		Target: &libvirtxml.StorageVolumeTarget{
			Format: &libvirtxml.StorageVolumeTargetFormat{
				Type: "qcow2",
			},
		},
	}

	var newVolume *libvirt.StorageVol

	if volumeTargetFormat == "qcow2" && linked {
		// For linked clones, we can use a backing store, which is much faster to create and
		// more storage-efficient. The downside being it is copy-on-write, which may be
		// less efficient at runtime, but is normally fine  for common tasks.
		backingStore := libvirtxml.StorageVolumeBackingStore{
			Path: disk.Source.File.File,
			Format: &libvirtxml.StorageVolumeTargetFormat{
				Type: volumeTargetFormat,
			},
		}
		newVolumeDescription.BackingStore = &backingStore

		if xmlDesc, err := xml.Marshal(&newVolumeDescription); err != nil {
			return nil, err
		} else if vol, err := backingPool.StorageVolCreateXML(string(xmlDesc), 0); err != nil {
			return nil, err
		} else {
			newVolume = vol
		}
	} else {
		// For non-linked clones, we just do a regular disk clone which recreates
		// and copies the entire volume. This can take a while... :(
		if xmlDesc, err := xml.Marshal(&newVolumeDescription); err != nil {
			return nil, err
		} else if vol, err := backingPool.StorageVolCreateXMLFrom(string(xmlDesc), backingVolume, 0); err != nil {
			return nil, err
		} else {
			newVolume = vol
		}
	}

	disk.Source.File.File, err = newVolume.GetPath()
	return newVolume, nil
}

func (dom *Domain) Snapshot(name string) error {
	return nil
}
