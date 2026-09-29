// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package diskutils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests the validity of the blockDeviceInfo struct's data modeling of size as `json.Number`
// with respect to the ability of the standard json.Unmarshal function to handle input from
// old and new versions of lsblk.
func TestValidBlockDevicesOutputSizeVariance(t *testing.T) {
	// The first device, with size as a number, is typical output for lsblk from util-linux >= v2.33
	// The second device, with size as a quoted string, is typical output for lsblk from util-linux < v2.33
	const validSizeVarianceJSON = `{
		"blockdevices": [
		   {"name":"nvme0n1", "size":1000204886016, "model":"Super Cool NVMe SSD 1TB", "maj:min":"259:1"},
		   {"name":"nvme1n1", "size":"1000204886016", "model":"Super Cool NVMe SSD 1TB", "maj:min":"259:1"}
		]
	 }`

	expectedBlockDevicesOutput := blockDevicesOutput{
		Devices: []blockDeviceInfo{
			{
				Name:   "nvme0n1",
				Size:   "1000204886016",
				Model:  "Super Cool NVMe SSD 1TB",
				MajMin: "259:1",
			},
			{
				Name:   "nvme1n1",
				Size:   "1000204886016",
				Model:  "Super Cool NVMe SSD 1TB",
				MajMin: "259:1",
			},
		},
	}

	var blockDevices blockDevicesOutput
	bytes := []byte(validSizeVarianceJSON)
	err := json.Unmarshal(bytes, &blockDevices)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedBlockDevicesOutput, blockDevices)
}

func TestFilterAndSortDiskPartitions(t *testing.T) {
	testCases := []struct {
		name        string
		diskPath    string
		inputPaths  []string
		wantedPaths []string
	}{
		{"empty", "/dev/loop0", nil, nil},
		{"disk only", "/dev/loop0", []string{"/dev/loop0"}, nil},
		{
			"reversed partitions", "/dev/loop0",
			[]string{"/dev/loop0", "/dev/loop0p2", "/dev/loop0p1"},
			[]string{"/dev/loop0p1", "/dev/loop0p2"},
		},
		{
			"disk last", "/dev/loop12",
			[]string{"/dev/loop12p2", "/dev/loop12p1", "/dev/loop12"},
			[]string{"/dev/loop12p1", "/dev/loop12p2"},
		},
		{
			"already sorted", "/dev/loop0",
			[]string{"/dev/loop0", "/dev/loop0p1", "/dev/loop0p2"},
			[]string{"/dev/loop0p1", "/dev/loop0p2"},
		},
		{
			"without disk entry", "/dev/loop0",
			[]string{"/dev/loop0p2", "/dev/loop0p1"},
			[]string{"/dev/loop0p1", "/dev/loop0p2"},
		},
		{
			"multi-digit partition numbers", "/dev/loop0",
			[]string{"/dev/loop0p10", "/dev/loop0p9", "/dev/loop0p8", "/dev/loop0p7", "/dev/loop0p6",
				"/dev/loop0p5", "/dev/loop0p4", "/dev/loop0p3", "/dev/loop0p2", "/dev/loop0p1", "/dev/loop0"},
			[]string{"/dev/loop0p1", "/dev/loop0p2", "/dev/loop0p3", "/dev/loop0p4",
				"/dev/loop0p5", "/dev/loop0p6", "/dev/loop0p7", "/dev/loop0p8", "/dev/loop0p9", "/dev/loop0p10"},
		},
		{
			"SCSI", "/dev/sda",
			[]string{"/dev/sda2", "/dev/sda", "/dev/sda1"},
			[]string{"/dev/sda1", "/dev/sda2"},
		},
		{
			"NVMe", "/dev/nvme0n1",
			[]string{"/dev/nvme0n1p2", "/dev/nvme0n1p1", "/dev/nvme0n1"},
			[]string{"/dev/nvme0n1p1", "/dev/nvme0n1p2"},
		},
		{
			"NBD", "/dev/nbd0",
			[]string{"/dev/nbd0p2", "/dev/nbd0", "/dev/nbd0p1"},
			[]string{"/dev/nbd0p1", "/dev/nbd0p2"},
		},
		{
			"MMC", "/dev/mmcblk0",
			[]string{"/dev/mmcblk0p2", "/dev/mmcblk0", "/dev/mmcblk0p1"},
			[]string{"/dev/mmcblk0p1", "/dev/mmcblk0p2"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			partitions := make([]PartitionInfo, len(testCase.inputPaths))
			for index, path := range testCase.inputPaths {
				deviceType := "part"
				if path == testCase.diskPath {
					deviceType = "disk"
				}
				partitions[index] = PartitionInfo{Path: path, Type: deviceType, PartUuid: path + "-uuid"}
			}

			partitions = filterAndSortDiskPartitions(partitions)
			if !assert.Len(t, partitions, len(testCase.wantedPaths)) {
				return
			}
			for index, path := range testCase.wantedPaths {
				assert.Equal(t, "part", partitions[index].Type)
				assert.Equal(t, path, partitions[index].Path)
				assert.Equal(t, path+"-uuid", partitions[index].PartUuid)
			}
		})
	}
}

func TestFilterAndSortDiskPartitionsRemovesNonPartitions(t *testing.T) {
	partitions := []PartitionInfo{
		{Path: "/dev/mapper/root", Type: "crypt"},
		{Path: "/dev/loop0p2", Type: "part"},
		{Path: "/dev/mapper/root-verity", Type: "dm"},
		{Path: "/dev/mapper/cryptVG-root", Type: "lvm"},
		{Path: "/dev/loop0", Type: "loop"},
		{Path: "/dev/loop0p1", Type: "part"},
	}

	partitions = filterAndSortDiskPartitions(partitions)
	assert.Equal(t, []PartitionInfo{
		{Path: "/dev/loop0p1", Type: "part"},
		{Path: "/dev/loop0p2", Type: "part"},
	}, partitions)
}
