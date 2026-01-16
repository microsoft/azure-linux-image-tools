package imagecustomizerapi

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// MountPoint holds the mounting information for each partition.
type MountPoint struct {
	// The ID type to use for the source in the /etc/fstab file.
	IdType MountIdentifierType `yaml:"idType" json:"idType,omitempty"`
	// The additional options for the mount.
	Options string `yaml:"options" json:"options,omitempty"`
	// The target directory path of the mount.
	Path string `yaml:"path" json:"path,omitempty"`
}

// UnmarshalYAML enables MountPoint to handle both a shorthand path and a structured object.
func (p *MountPoint) UnmarshalYAML(value *yaml.Node) error {
	// Check if the node is a scalar (i.e., single path string).
	if value.Kind == yaml.ScalarNode {
		// Treat scalar value as the Path directly.
		p.Path = value.Value
		return nil
	}

	// yaml.Node.Decode() doesn't respect the KnownFields() option.
	// So, manually enforce this.
	err := checkKnownFields(value, "MountPoint", []string{"idType", "options", "path"})
	if err != nil {
		return err
	}

	// Otherwise, decode as a full MountPoint struct.
	type IntermediateTypeMountPoint MountPoint
	err = value.Decode((*IntermediateTypeMountPoint)(p))
	if err != nil {
		return fmt.Errorf("failed to parse MountPoint struct:\n%w", err)
	}
	return nil
}

// IsValid returns an error if the MountPoint is not valid
func (p *MountPoint) IsValid() error {
	err := p.IdType.IsValid()
	if err != nil {
		return fmt.Errorf("invalid idType value:\n%w", err)
	}

	// Use validatePath to check the Path field.
	if err := validatePath(p.Path); err != nil {
		return fmt.Errorf("invalid path:\n%w", err)
	}

	// Use validateMountOptions to check Options.
	if validateMountOptions(p.Options) {
		return fmt.Errorf("options (%s) contain spaces, tabs, or newlines and are invalid", p.Options)
	}

	return nil
}

// validateBtrfsMountOptions validates that mount options for BTRFS filesystems and subvolumes
// don't contain subvol= or subvolid=.
func validateBtrfsMountOptions(options string) error {
	if options == "" {
		return nil
	}

	optionsList := strings.Split(options, ",")
	for _, opt := range optionsList {
		// No need to trim spaces here.
		// The kernel will not recognize whitespace before or after option names, or around '='.
		if strings.HasPrefix(opt, "subvol=") {
			return fmt.Errorf("'subvol=' option is not allowed; it is automatically added by Image Customizer")
		}
		if strings.HasPrefix(opt, "subvolid=") {
			return fmt.Errorf("'subvolid=' option is not allowed; 'subvol=' is automatically added by Image Customizer")
		}
	}

	return nil
}
