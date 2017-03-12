package provider

//
//import (
//	"path"
//	"sync"
//	"os"
//	"io/ioutil"
//	"encoding/json"
//	"fmt"
//	"strings"
//)
//
//type FileStore struct {
//	mutex   sync.Mutex
//	basedir string
//}
//
//func NewFileStore(basedir string) (*FileStore) {
//	return &FileStore{basedir:basedir}
//}
//var _ Store = &FileStore{}
//
//func (f*FileStore) ListZones() ([]string, error) {
//	f.mutex.Lock()
//	defer f.mutex.Unlock()
//
//	files, err := ioutil.ReadDir(f.basedir)
//	if err != nil {
//		return nil, err
//	}
//
//	var names []string
//	for _, f := range files {
//		names = append(names, f.Name())
//	}
//	return names, nil
//}
//
//func (f*FileStore) LoadZone(parent *Zones, zoneID string) (*Zone, error) {
//	f.mutex.Lock()
//	defer f.mutex.Unlock()
//
//	p := f.pathForZone(zoneID)
//	data, err := ioutil.ReadFile(p)
//	if err != nil {
//		return nil, fmt.Errorf("unable to read file %q backing zone %s: %v", p, zoneID, err)
//	}
//
//	recordMap := make(map[string]*ResourceRecordSet)
//	for _, line := range strings.Split(string(data), "\n") {
//		line = strings.TrimSpace(line)
//		if line == "" {
//			continue
//		}
//
//		record := &ResourceRecordSet{}
//		err := json.Unmarshal([]byte(line), record)
//		if err != nil {
//			return nil, fmt.Errorf("unable to read file %q backing zone %s: %v", p, zoneID, err)
//		}
//
//		recordMap[keyForResourceRecordSet(record)] = record
//	}
//
//	return &Zone{
//		parent:  parent,
//		name:    zoneID,
//		records: recordMap,
//	}, nil
//}
//
//func (f*FileStore) UpdateZone(zone *Zone) error {
//	f.mutex.Lock()
//	defer f.mutex.Unlock()
//
//
//	var lines []string
//	for _, rrs := range zone.records {
//		json, err := json.Marshal(rrs.data)
//		if err != nil {
//			return err
//		}
//		lines = append(lines, string(json))
//	}
//
//	p := f.pathForZone(zone.ID())
//	zoneID := zone.ID()
//	data := []byte(strings.Join(lines, "\n"))
//	 err := ioutil.WriteFile(p, data, 0644)
//	if err != nil {
//		return fmt.Errorf("unable to write file %q backing zone %s: %v", p, zoneID, err)
//	}
//	return nil
//}
//
//func (f*FileStore) RemoveZone(zone *Zone) error {
//	p := f.pathForZone(zone.ID())
//	err := os.Remove(p)
//	if err != nil {
//		if os.IsNotExist(err) {
//			return nil
//		}
//		return err
//	}
//	return nil
//}
//
//func (f*FileStore) pathForZone(zoneID string) string {
//	return path.Join(f.basedir, zoneID)
//}
