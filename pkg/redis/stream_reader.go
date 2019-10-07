package redis

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

type Componenter interface {
}

type redisArray []Componenter

type redisInt int

func (r redisInt) Int() int {
	return int(r)
}

type redisString string

func (r redisString) String() string {
	return string(r)
}

func ComponentFromReader(reader io.Reader, buffer []byte) (component Componenter, bytesReadTotal int, err error) {
	var fieldType [1]byte
	var bytesRead int
	bytesRead, err = reader.Read(fieldType[:])
	if err != nil {
		return
	}
	if bytesRead != 1 {
		return nil, bytesReadTotal, fmt.Errorf("read too few bytes when trying to determine the field type")
	}
	bytesReadTotal += bytesRead
	switch fieldType[0] {
	case '*': // array
		var arrayLenStr string
		arrayLenStr, bytesRead, err = readFieldAsString(reader, buffer)
		if err != nil {
			return
		}
		bytesReadTotal += bytesRead
		var arrayLength int
		arrayLength, err = strconv.Atoi(arrayLenStr)
		if err != nil {
			return
		}
		array := make([]Componenter, arrayLength)
		for componentIndex := range array {
			array[componentIndex], bytesRead, err = ComponentFromReader(reader, buffer)
			bytesReadTotal += bytesRead
		}
		arrayForReference := redisArray(array)
		return &arrayForReference, bytesReadTotal, nil
	case ':': // integer
		var asInt int
		asInt, bytesRead, err = readFieldAsInt(reader, buffer)
		if err != nil {
			return
		}
		bytesReadTotal += bytesRead
		intForReference := redisInt(asInt)
		return &intForReference, bytesReadTotal, nil
	case '$': // bulk string
		var strLen int
		strLen, bytesRead, err = readFieldAsInt(reader, buffer)
		if err != nil {
			return
		}
		bytesReadTotal += bytesRead
		if strLen > cap(buffer) {
			return nil, bytesReadTotal, fmt.Errorf("unable to read bulk-string with length %d as this exceeds our buffersize of %d", strLen, cap(buffer))
		}
		var theString string
		theString, bytesRead, err = readFieldAsString(reader, buffer)
		if err != nil {
			return nil, bytesReadTotal, fmt.Errorf("unable to read Bulk String at offset: %d", bytesReadTotal)
		}
		bytesReadTotal += bytesRead
		stringForReference := redisString(theString)
		return &stringForReference, bytesReadTotal, nil
	case '+': // simple string
		var theString string
		theString, bytesRead, err = readFieldAsString(reader, buffer)
		if err != nil {
			return nil, bytesReadTotal, fmt.Errorf("unable to read Simple String at offset: %d", bytesReadTotal)
		}
		bytesReadTotal += bytesRead
		stringForReference := redisString(theString)
		return &stringForReference, bytesReadTotal, nil
	default:
		return nil, bytesReadTotal, fmt.Errorf("unrecognized field type: '%c'", fieldType[0])
	}
}

var RecordEndMarker = []byte{'\r', '\n'}

func readFieldAsString(reader io.Reader, buffer []byte) (fieldValue string, bytesReadTotal int, err error) {
	buffer = resetBuffer(buffer)
	var bytesRead int
	for bytesReadTotal < cap(buffer) {
		bytesRead, err = reader.Read(buffer[bytesReadTotal : bytesReadTotal+1])
		if err != nil {
			return
		}
		if bytesRead != 1 {
			return "", bytesReadTotal, fmt.Errorf("expected to read value, but did not read anything")
		}
		bytesReadTotal += bytesRead
		if bytesReadTotal > 1 && bytes.Compare(RecordEndMarker, buffer[bytesReadTotal-len(RecordEndMarker):bytesReadTotal]) == 0 {
			break
		}
	}
	return string(buffer[0 : bytesReadTotal-len(RecordEndMarker)]), bytesReadTotal, nil
}

func readFieldAsInt(reader io.Reader, buffer []byte) (fieldValue int, bytesReadTotal int, err error) {
	var intStr string
	intStr, bytesReadTotal, err = readFieldAsString(reader, buffer)
	if err != nil {
		return
	}
	fieldValue, err = strconv.Atoi(intStr)
	if err != nil {
		return
	}
	return
}

func resetBuffer(buffer []byte) []byte {
	return buffer[0:cap(buffer)]
}
