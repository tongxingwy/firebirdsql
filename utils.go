/*******************************************************************************
The MIT License (MIT)

Copyright (c) 2013-2014 Hajime Nakagami

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*******************************************************************************/

package firebirdsql

import (
	"bytes"
	"container/list"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func str_to_bytes(s string) []byte {
	return bytes.NewBufferString(s).Bytes()
}

func int32_to_bytes(i32 int32) []byte {
	bs := []byte{
		byte(i32 & 0xFF),
		byte(i32 >> 8 & 0xFF),
		byte(i32 >> 16 & 0xFF),
		byte(i32 >> 24 & 0xFF),
	}
	return bs
}

func bint32_to_bytes(i32 int32) []byte {
	bs := []byte{
		byte(i32 >> 24 & 0xFF),
		byte(i32 >> 16 & 0xFF),
		byte(i32 >> 8 & 0xFF),
		byte(i32 & 0xFF),
	}
	return bs
}

func int16_to_bytes(i16 int16) []byte {
	bs := []byte{
		byte(i16 & 0xFF),
		byte(i16 >> 8 & 0xFF),
	}
	return bs
}
func bytes_to_str(b []byte) string {
	return bytes.NewBuffer(b).String()
}

func bytes_to_bint32(b []byte) int32 {
	var i32 int32
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.BigEndian, &i32)
	return i32
}

func bytes_to_int32(b []byte) int32 {
	var i32 int32
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.LittleEndian, &i32)
	return i32
}

func bytes_to_bint16(b []byte) int16 {
	var i int16
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.BigEndian, &i)
	return i
}

func bytes_to_int16(b []byte) int16 {
	var i int16
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.LittleEndian, &i)
	return i
}

func bytes_to_bint64(b []byte) int64 {
	var i int64
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.BigEndian, &i)
	return i
}

func bytes_to_int64(b []byte) int64 {
	var i int64
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.LittleEndian, &i)
	return i
}

func xdrBytes(bs []byte) []byte {
	// XDR encoding bytes
	n := len(bs)
	padding := 0
	if n%4 != 0 {
		padding = 4 - n%4
	}
	buf := make([]byte, 4+n+padding)
	buf[0] = byte(n >> 24 & 0xFF)
	buf[1] = byte(n >> 16 & 0xFF)
	buf[2] = byte(n >> 8 & 0xFF)
	buf[3] = byte(n & 0xFF)
	for i, b := range bs {
		buf[4+i] = b
	}
	return buf
}

func xdrString(s string) []byte {
	// XDR encoding string
	bs := bytes.NewBufferString(s).Bytes()
	return xdrBytes(bs)
}

func flattenBytes(l *list.List) []byte {
	n := 0
	for e := l.Front(); e != nil; e = e.Next() {
		n += len((e.Value).([]byte))
	}

	bs := make([]byte, n)

	n = 0
	for e := l.Front(); e != nil; e = e.Next() {
		for i, b := range (e.Value).([]byte) {
			bs[n+i] = b
		}
		n += len((e.Value).([]byte))
	}

	return bs
}

func _int32ToBlr(i32 int32) ([]byte, []byte) {
	v := bytes.Join([][]byte{
		bint32_to_bytes(i32),
	}, nil)
	blr := []byte{8, 0}

	return blr, v
}

func _strToBlr(s string) ([]byte, []byte) {
	v := str_to_bytes(s)
	nbytes := len(v)
	pad_length := ((4 - nbytes) & 3)
	padding := make([]byte, pad_length)
	v = bytes.Join([][]byte{
		v,
		padding,
	}, nil)
	blr := []byte{14, byte(nbytes & 255), byte(nbytes >> 8)}
	return blr, v
}

func _convert_date(t time.Time) []byte {
	i := int(t.Month()) + 9
	jy := t.Year() + (i / 12) - 1
	jm := i % 12
	c := jy / 100
	jy -= 100 * c
	j := (146097*c)/4 + (1461*jy)/4 + (153*jm+2)/5 + t.Day() - 678882
	return bint32_to_bytes(int32(j))
}

func _convert_time(t time.Time) []byte {
	v := (t.Hour()*3600+t.Minute()*60+t.Second())*10000 + t.Nanosecond()/100000
	return bint32_to_bytes(int32(v))
}

func _dateToBlr(t time.Time) ([]byte, []byte) {
	v := bytes.Join([][]byte{
		_convert_date(t),
	}, nil)
	blr := []byte{12}
	return blr, v
}

func _timeToBlr(t time.Time) ([]byte, []byte) {
	v := bytes.Join([][]byte{
		_convert_time(t),
	}, nil)
	blr := []byte{13}
	return blr, v
}

func _timestampToBlr(t time.Time) ([]byte, []byte) {
	v := bytes.Join([][]byte{
		_convert_date(t),
		_convert_time(t),
	}, nil)

	blr := []byte{35}
	return blr, v
}

func paramsToBlr(params []driver.Value, protocolVersion int32) ([]byte, []byte) {
	// Convert parameter array to BLR and values format.
	var v, blr []byte
	bi256 := big.NewInt(256)

	ln := len(params) * 2
	blrList := list.New()
	valuesList := list.New()
	blrList.PushBack([]byte{5, 2, 4, 0, byte(ln & 255), byte(ln >> 8)})

	if protocolVersion >= PROTOCOL_VERSION13 {
		null_indicator := new(big.Int)
		for i := len(params) - 1; i > 0; i-- {
			if params[i] == nil {
				null_indicator.SetBit(null_indicator, i, 1)
			}
		}
		n := len(params) / 8
		if len(params)%8 != 0 {
			n++
		}
		if n%4 != 0 { // padding
			n += 4 - n%4
		}
		for i := 0; i < n; i++ {
			valuesList.PushBack([]byte{byte(null_indicator.Mod(null_indicator, bi256).Int64())})
			null_indicator = null_indicator.Div(null_indicator, bi256)
		}
	}

	for _, p := range params {
		switch f := p.(type) {
		case string:
			blr, v = _strToBlr(f)
		case int:
			blr, v = _int32ToBlr(int32(f))
		case int16:
			blr, v = _int32ToBlr(int32(f))
		case int32:
			blr, v = _int32ToBlr(f)
		case int64:
			blr, v = _int32ToBlr(int32(f))
		case time.Time:
			if f.Year() == 0 {
				blr, v = _timeToBlr(f)
			} else {
				blr, v = _timestampToBlr(f)
			}
		case bool:
			if f {
				v = []byte{1, 0, 0, 0}
			} else {
				v = []byte{0, 0, 0, 0}
			}
			blr = []byte{23}
		case nil:
			v = []byte{}
			blr = []byte{14, 0, 0}
		default:
			// can't convert directory
			blr, v = _strToBlr(fmt.Sprintf("%v", f))
		}
		valuesList.PushBack(v)
		if protocolVersion < PROTOCOL_VERSION13 {
			if p == nil {
				valuesList.PushBack([]byte{0xff, 0xff, 0xff, 0xff})
			} else {
				valuesList.PushBack([]byte{0, 0, 0, 0})
			}
		}
		blrList.PushBack(blr)
		blrList.PushBack([]byte{7, 0})
	}
	blrList.PushBack([]byte{255, 76}) // [blr_end, blr_eoc]

	blr = flattenBytes(blrList)
	v = flattenBytes(valuesList)

	return blr, v
}

func split1(src string, delm string) (string, string) {
	for i := 0; i < len(src); i++ {
		if src[i:i+1] == delm {
			s1 := src[0:i]
			s2 := src[i+1:]
			return s1, s2
		}
	}
	return src, ""
}

func parseDSN(dsn string) (addr string, dbName string, user string, passwd string, role string, err error) {
	s1, s2 := split1(dsn, "@")
	user, passwd = split1(s1, ":")
	passwd, role = split1(passwd, ":")
	addr, dbName = split1(s2, "/")
	if !strings.ContainsRune(addr, ':') {
		addr += ":3050"
	}
	if strings.ContainsRune(dbName, '/') {
		dbName = "/" + dbName
	}

	return
}

func calcBlr(xsqlda []xSQLVAR) []byte {
	// Calculate  BLR from XSQLVAR array.
	ln := len(xsqlda) * 2
	blr := make([]byte, (ln*4)+8)
	blr[0] = 5
	blr[1] = 2
	blr[2] = 4
	blr[3] = 0
	blr[4] = byte(ln & 255)
	blr[5] = byte(ln >> 8)
	n := 6

	for _, x := range xsqlda {
		sqlscale := x.sqlscale
		if sqlscale < 0 {
			sqlscale += 256
		}
		switch x.sqltype {
		case SQL_TYPE_VARYING:
			blr[n] = 37
			blr[n+1] = byte(x.sqllen & 255)
			blr[n+2] = byte(x.sqllen >> 8)
			n += 3
		case SQL_TYPE_TEXT:
			blr[n] = 14
			blr[n+1] = byte(x.sqllen & 255)
			blr[n+2] = byte(x.sqllen >> 8)
			n += 3
		case SQL_TYPE_LONG:
			blr[n] = 8
			blr[n+1] = byte(sqlscale)
			n += 2
		case SQL_TYPE_SHORT:
			blr[n] = 7
			blr[n+1] = byte(sqlscale)
			n += 2
		case SQL_TYPE_INT64:
			blr[n] = 16
			blr[n+1] = byte(sqlscale)
			n += 2
		case SQL_TYPE_QUAD:
			blr[n] = 9
			blr[n+1] = byte(sqlscale)
			n += 2
		case SQL_TYPE_BLOB:
			blr[n] = 9
			blr[n+1] = 0
			n += 2
		case SQL_TYPE_ARRAY:
			blr[n] = 9
			blr[n+1] = 0
			n += 2
		case SQL_TYPE_DOUBLE:
			blr[n] = 27
			n += 1
		case SQL_TYPE_FLOAT:
			blr[n] = 10
			n += 1
		case SQL_TYPE_D_FLOAT:
			blr[n] = 11
			n += 1
		case SQL_TYPE_DATE:
			blr[n] = 12
			n += 1
		case SQL_TYPE_TIME:
			blr[n] = 13
			n += 1
		case SQL_TYPE_TIMESTAMP:
			blr[n] = 35
			n += 1
		case SQL_TYPE_BOOLEAN:
			blr[n] = 23
			n += 1
		}
		// [blr_short, 0]
		blr[n] = 7
		blr[n+1] = 0
		n += 2
	}
	// [blr_end, blr_eoc]
	blr[n] = 255
	blr[n+1] = 76
	n += 2

	return blr[:n]
}
