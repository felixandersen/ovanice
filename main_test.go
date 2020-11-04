package main

import (
	"archive/tar"
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestOvaFileDefinition(t *testing.T) {
		ovaFile := getOvaFileDefinition("./", "minimal_test.ova")
		if ovaFile.Name != "minimal_test.ova" {
			t.Errorf("ovaFile name was incorrect, got: %s, want: %s.", ovaFile.Name, "minimal_test.ova")
		}

		expected := []virtualHardwareItem{
			{
				ElementName:"16 virtual CPU(s)",
				Description: "Number of Virtual CPUs",
				ResourceType: 3,
				VirtualQuantity: 16,
				Param: "cores",
			}, {
				ElementName: "16384MB of memory",
				Description: "Memory Size",
				ResourceType: 4,
				VirtualQuantity: 16,
				Param: "ram",
			},
		}
		value := ovaFile.VirtualHardwareItems
		if !reflect.DeepEqual(value, expected) {
			t.Errorf("ovaFile VirtualHardwareItems was incorrect, got: %+v want: %+v", value, expected)
		}
}

func TestCreateOvaFile(t *testing.T) {
	ovaFile, err := createOvaFile("minimal_test.ova", *bytes.NewBufferString("test ovf content"))
	check(err)

	buf := bytes.NewBuffer(ovaFile)
  rdr := tar.NewReader(buf);

	for {
		hdr, err := rdr.Next()
		if err == io.EOF {
			break // End of archive
		}
		check(err)
		if hdr.Name == "minimal_test.mf" {
			expected := "SHA256(minimal_test.ovf)= 50650eb132cb8ef3288cb03d921a7a0c6a058c00e8f7a2f3ae0c4d6f5ccea4e3\nSHA256(minimal_test.vmdk)= e907dc3b33e445d517c2887ab53e8ac22a4ab6bd193e2430841d7b41464edd0d"
			buf := new(strings.Builder)
			_, err := io.Copy(buf, rdr)
			check(err)
			value := buf.String()
			if !reflect.DeepEqual(value, expected) {
				t.Errorf("CreateOvaFile generated manifest file was was incorrect, got: %+v want: %+v", value, expected)
			}
		} else if hdr.Name == "minimal_test.vmdk" {
			expected := "this is a test disk"
			buf := new(strings.Builder)
			_, err := io.Copy(buf, rdr)
			check(err)
			value := buf.String()
			if !reflect.DeepEqual(value, expected) {
				t.Errorf("CreateOvaFile generated disk file was was incorrect, got: %+v want: %+v", value, expected)
			}
		} else if hdr.Name == "minimal_test.ovf" {
			expected := "test ovf content"
			buf := new(strings.Builder)
			_, err := io.Copy(buf, rdr)
			check(err)
			value := buf.String()
			if !reflect.DeepEqual(value, expected) {
				t.Errorf("CreateOvaFile generated ovf file was was incorrect, got: %+v want: %+v", value, expected)
			}
		} else {
			t.Errorf("CreateOvaFile generated an unexpected file: %+v", hdr.Name)
		}
	}
}
