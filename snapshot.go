package rex

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cheekybits/genny/generic"
	"github.com/remerge/gobi"
)

type _T_ generic.Type

// type gobi generic.Type

type SnapshoterFor_T_ struct {
	basePath   string
	ext        string
	timeLayout string
}

var _T_EncodeGauge = ""
var _T_DecodeGauge = ""

var _T_Snapshoter *SnapshoterFor_T_ = NewSnapshoterFor_T_("_basePath", "_ext", "_timeLayout")

func NewSnapshoterFor_T_(basePath, ext, timeLayout string) *SnapshoterFor_T_ {
	return &SnapshoterFor_T_{basePath, ext, timeLayout}
}

func (self *SnapshoterFor_T_) BasePath() string {
	path, err := filepath.Abs(self.basePath)
	_MayPanic(err)
	return path
}

func (self *SnapshoterFor_T_) Path(name string) string {
	return filepath.Join(self.BasePath(), name)
}

func (self *SnapshoterFor_T_) Glob() ([]string, error) {
	if len(self.ext) > 0 {
		return filepath.Glob(self.Path("*." + self.ext))
	} else {
		return filepath.Glob(self.Path("*"))
	}
}

func (self *SnapshoterFor_T_) Newest() (string, error) {
	files, err := self.sorted()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("No snapshot found in " + self.BasePath())
	}
	return files[len(files)-1], nil
}

// explicit setter
func (self *SnapshoterFor_T_) SetExt(n string) {
	self.ext = n
}

// func (self *SnapshoterFor_T_) Offset(file string) int64 {
// 	pattern := regexp.MustCompile(`\-(\d+)(\-\d+)?\.` + self.Ext + `$`)
// 	r := pattern.FindStringSubmatch(file)
// 	if len(r) < 2 {
// 		_MayPanic(errors.New("invalid filename passed to SnapshotOffseter:" + file))
// 	}
// 	i, err := strconv.Atoi(r[1])
// 	_MayPanic(err)
// 	return int64(i)
// }

// func (self *SnapshoterFor_T_) NewestOffsetSafe() int64 {
// 	defer func() {
// 		if r := recover(); r != nil {
// 		}
// 	}()
// 	file, err := self.Newest()
// 	if err != nil {
// 		return -1
// 	}
// 	return self.Offset(file)
// }

func (self *SnapshoterFor_T_) sorted() ([]string, error) {
	files, err := self.Glob()
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func removeFiles(files []string) (deleted []string, errs []error) {
	if len(files) == 0 {
		return []string{}, nil
	}
	deleted = make([]string, 0)
	errs = make([]error, 0)
	for _, file := range files {
		err := os.Remove(file)
		if err == nil {
			deleted = append(deleted, file)
		} else {
			errs = append(errs, errors.New(fmt.Sprintf("not deleted %s : %v", file, err)))
		}
	}
	return deleted, errs
}

// clean all but the newest
func (self *SnapshoterFor_T_) Clean() ([]string, []error) {
	files, err := self.sorted()
	_MayPanic(err)
	if len(files) <= 1 {
		return []string{}, nil
	}
	return removeFiles(files[0 : len(files)-1])
}

func (self *SnapshoterFor_T_) CleanAll() ([]string, []error) {
	files, err := self.Glob()
	_MayPanic(err)
	return removeFiles(files[0 : len(files)-1])
}

func (self *SnapshoterFor_T_) Load(b []byte) (*_G_, error) {
	buf := bytes.NewBuffer(b)
	return self.LoadFromReader(buf)
}

func (self *SnapshoterFor_T_) LoadFromReader(r io.Reader) (result *_G_, err error) {
	dec := gobi.NewDecoder(r)
	dec.SetupSizeMetrics(_T_DecodeGauge)

	result = new(_G_)
	err = dec.Decode(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (self *SnapshoterFor_T_) LoadFromReaderAsInterface(r io.Reader) (result interface{}, err error) {
	return self.LoadFromReader(r)
}

func (self *SnapshoterFor_T_) LoadFromFile(filename string) (result *_G_, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		_MayPanic(file.Close())
	}()
	return self.LoadFromReader(file)
}

func (self *SnapshoterFor_T_) LoadNewestFromFile() (result *_G_, err error) {
	file, err := self.Newest()
	if err != nil {
		return nil, err
	}
	return self.LoadFromFile(file)
}

func (self *SnapshoterFor_T_) LoadNewestFromFileAsInterface() (result interface{}, err error) {
	return self.LoadNewestFromFile()
}

func (self *SnapshoterFor_T_) BytesToFileNamedByTime(b []byte) (string, error) {
	filename := _T_Snapshoter.CurrentTimeBasedFilename()
	err := self.BytesToFile(b, _T_Snapshoter.CurrentTimeBasedFilename())

	return filename, err
}

func (self *SnapshoterFor_T_) BytesToFile(b []byte, filename string) error {
	dir := filepath.Dir(filename)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	fo, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_MayPanic(fo.Close())
	}()
	w := bufio.NewWriter(fo)
	if _, err = w.Write(b); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	return nil

}

func (self *SnapshoterFor_T_) CurrentTimeBasedFilename() string {
	rpath := fmt.Sprintf("%s.%s", time.Now().Format(self.timeLayout), self.ext)
	return self.Path(rpath)
}

func (self *SnapshoterFor_T_) Filename(date time.Time) string {
	return fmt.Sprintf("%s.%s", date.Format(self.timeLayout), self.ext)
}

func (self *SnapshoterFor_T_) AbsFilename(date time.Time) string {
	return self.Path(self.Filename(date))
}

func (self *_G_) SnapshotToFileNamedBy(time time.Time) (err error) {
	return self.SnapshotToFile(_T_Snapshoter.AbsFilename(time))
}

func (self *_G_) SnapshotToFileNamedByTime() (filename string, err error) {
	filename = _T_Snapshoter.CurrentTimeBasedFilename()
	err = self.SnapshotToFile(filename)
	return filename, err
}

func (self *_G_) SnapshotToWriter(w io.Writer) {
	enc := gobi.NewEncoder(w)
	enc.SetupSizeMetrics(_T_EncodeGauge)

	err := enc.Encode(self)
	_MayPanic(err)
}

func (self *_G_) SnapshotToFile(filename string) error {
	dir := filepath.Dir(filename)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	fo, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_MayPanic(fo.Close())
	}()
	w := bufio.NewWriter(fo)
	self.SnapshotToWriter(w)
	if err = w.Flush(); err != nil {
		return err
	}
	return nil
}

func (self *_G_) Snapshot() []byte {
	var result bytes.Buffer
	self.SnapshotToWriter(&result)
	return result.Bytes()
}
