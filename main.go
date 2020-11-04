package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/beevik/etree"
	"github.com/gin-gonic/gin"
)

type description struct {
	Name    string `xml:"name"`
	Version string `xml:"version"`
}

type item struct {
    AllocationUnits string `xml:"AllocationUnits"`
    Description string `xml:"Description"`
    ElementName string `xml:"ElementName"`
    InstanceID  string `xml:"InstanceID"`
    ResourceType    string `xml:"ResourceType"`
    VirtualQuantity string `xml:"VirtualQuantity"`
}

func check(e error) {
    if e != nil {
        debug.PrintStack()
        log.Fatal(e)
    }
}

func ovfFile(ovaFilename string) (reader *tar.Reader, err error) {
    dat, err := ioutil.ReadFile(ovaFilename)
    check(err)
    var buf = bytes.NewBuffer(dat)
    rdr := tar.NewReader(buf);

    for {
        hdr, err := rdr.Next()
        if err == io.EOF {
            break // End of archive
        }
        check(err)
        if strings.HasSuffix(hdr.Name, ".ovf") {
            return rdr, nil
        }
    }
    return nil, fmt.Errorf("ovfFile: no ovf-file in archive")
}

// CreateOvaFile takes an ova file, replaces the ovf in it, recalculates checksums in the manifest file and returns the new ova file
func createOvaFile(ovaFilename string, ovfFileBytesBuffer bytes.Buffer) (ovaFile []byte, err error) {
    ovfFile := bytes.NewReader(ovfFileBytesBuffer.Bytes())
    dat, err := ioutil.ReadFile(ovaFilename)
    check(err)
    readBuffer := bytes.NewBuffer(dat)
    reader := tar.NewReader(readBuffer);
    var writeBuffer bytes.Buffer
    writer := tar.NewWriter(&writeBuffer)

    type ManifestEntry struct {
        name string
        hash string
    }

    manifestEntries := []ManifestEntry{}
    var manifestFileName string;
    var manifestFileMode int64;
    for {
        hdr, err := reader.Next()
        if err == io.EOF {
            break // End of archive
        }
        check(err)
        if strings.HasSuffix(hdr.Name, ".ovf") {
            check(err)
            header := &tar.Header{
                Name: hdr.Name,
                Mode: hdr.Mode,
                Size: int64(ovfFile.Len()),
            }

            err = writer.WriteHeader(header)
            check(err);

            bufForSum := &bytes.Buffer{}

            multiwriter := io.MultiWriter(writer, bufForSum)

            _, err = io.Copy(multiwriter, ovfFile) 
            check(err)

            sum := sha256.Sum256(bufForSum.Bytes())
            hash := hex.EncodeToString(sum[:])
            manifestEntries = append(manifestEntries, ManifestEntry{name: hdr.Name, hash: hash})
        } else if strings.HasSuffix(hdr.Name, ".mf") {
            // we will add this file later, no-op
            manifestFileName = hdr.Name;
        } else {
            err = writer.WriteHeader(hdr)
            check(err);

            bufForSum := &bytes.Buffer{}

            multiwriter := io.MultiWriter(writer, bufForSum)

            _, err = io.Copy(multiwriter, reader);
            check(err)

            sum := sha256.Sum256(bufForSum.Bytes())
            hash := hex.EncodeToString(sum[:])
            manifestEntries = append(manifestEntries, ManifestEntry{name: hdr.Name, hash: hash})
        }
    }

    manifestFileContent := []string{}
    for _, item := range manifestEntries {
        manifestFileContent = append(manifestFileContent, strings.Join([]string{
            "SHA256(",
            item.name,
            ")= ",
            item.hash,
        }, ""))
    }
    completeManifestFile := strings.Join(manifestFileContent, "\n")

    writer.WriteHeader(&tar.Header{
        Name: manifestFileName,
        Mode: manifestFileMode,
        Size: int64(len(completeManifestFile)),
    })

    io.WriteString(writer, completeManifestFile)

    check(writer.Close())
    return writeBuffer.Bytes(), nil
}

type virtualHardwareItem struct {
    ElementName string
    Description string
    ResourceType int
    VirtualQuantity int
    Param string `json:"param"`
}

type ovaFileDefinition struct {
    Name string `json:"name"`;
    VirtualHardwareItems []virtualHardwareItem `json:"virtual_hardware_items"`;
    ExampleRequest string `json:"example_request"`
}

func processHardwareItems(file io.Reader, handler func(*etree.Element)) bytes.Buffer {
    doc := etree.NewDocument()
    buf, err := ioutil.ReadAll(file)
    check(err)
    err = doc.ReadFromBytes(buf)
    check(err)

    items := doc.FindElements("Envelope/VirtualSystem/VirtualHardwareSection/Item")
    for _, item := range items {
        qty := item.FindElement("VirtualQuantity")
        if qty != nil {
            handler(item)
        }
    }

    bts, err := doc.WriteToBytes()
    check(err)
    return *bytes.NewBuffer(bts)
}

func getOvaFileDefinition(filename string) ovaFileDefinition {
    openedFile, err := ovfFile(filename)
    check(err)
    
    exampleRequest := strings.Join([]string{"/ova_files", filename}, "/")
    exampleParams := []string{}
    hardwareItems := []virtualHardwareItem{}
    handler := func(item *etree.Element) {
        qty, err := strconv.Atoi(item.FindElement("VirtualQuantity").Text())
        check(err)
        resourceType, err := strconv.Atoi(item.FindElement("ResourceType").Text())
        check(err)
        elementName := item.FindElement("ElementName").Text()
        description := item.FindElement("Description").Text()
        var param string
        var exampleValue string
        if resourceType == 3 {
            param = "cores"
            exampleValue = "4"
        } else if resourceType == 4 {
            param = "ram"
            exampleValue = "2048"
        }
        hardwareItems = append(hardwareItems, virtualHardwareItem{
            ResourceType: resourceType,
            VirtualQuantity: qty,
            ElementName: elementName,
            Description: description,
            Param: param,
        })
        exampleParams = append(exampleParams, strings.Join([]string{param, exampleValue}, "="))
    }

    processHardwareItems(openedFile, handler)

    if len(exampleParams) > 0 {
        exampleRequest = strings.Join([]string{
            exampleRequest,
            strings.Join(exampleParams, "&"),
        }, "?")
    }

    f := ovaFileDefinition{Name: filename, VirtualHardwareItems: hardwareItems, ExampleRequest: exampleRequest}

    return f
}

func main() {
    r := gin.Default()
    r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
    })
    r.GET("/inventory", func(c *gin.Context) {
        file, err := os.Open("./")
        check(err)
        filenames, err := file.Readdirnames(0)
        check(err)
        
        ovaFiles := []ovaFileDefinition{}
        for _, filename := range filenames {
            if strings.HasSuffix(filename, ".ova") {
                f := getOvaFileDefinition(filename)
                ovaFiles = append(ovaFiles, f)
            }
        }
        c.PureJSON(200, gin.H{
            "ova_files": ovaFiles,
        })
    })
    r.GET("/ova_files/:file", func(c *gin.Context) {
        filenameComponents := strings.Split(c.Param("file"), "/")
        filename := filenameComponents[len(filenameComponents) - 1]
        if strings.HasSuffix(filename, ".ova") {
            openedFile, err := ovfFile(filename)
            check(err)
            
            handler := func(item *etree.Element) {
                resourceType, err := strconv.Atoi(item.FindElement("ResourceType").Text())
                check(err)
                if resourceType == 3 {
                    coresValue, err := strconv.Atoi(c.Query("cores"))
                    check(err)
                    item.FindElement("VirtualQuantity").SetText(strconv.Itoa(coresValue))
                } else if resourceType == 4 {
                    ramValue, err := strconv.Atoi(c.Query("ram"))
                    check(err)
                    item.FindElement("VirtualQuantity").SetText(strconv.Itoa(ramValue))
                }
            }

            ovfFile := processHardwareItems(openedFile, handler)

            newOvaFile, err := createOvaFile(filename, ovfFile)
            check(err)

            c.Header("Content-Description", "File Transfer")
            c.Header("Content-Disposition", "attachment; filename="+filename)
            c.Data(http.StatusOK, "application/octet-stream", newOvaFile)
        } else {

        }
    })

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
