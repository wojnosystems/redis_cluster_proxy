package redis

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

const RecordSeparator = "\r\n"

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
		arrayLenStr, bytesRead, err = readFieldAsSimpleString(reader, buffer)
		if err != nil {
			return
		}
		bytesReadTotal += bytesRead
		var arrayLength int
		arrayLength, err = strconv.Atoi(arrayLenStr)
		if err != nil {
			return
		}
		if arrayLength < 0 {
			return NewNull(), bytesReadTotal, nil
		}
		array := make([]Componenter, arrayLength)
		for componentIndex := range array {
			array[componentIndex], bytesRead, err = ComponentFromReader(reader, buffer)
			bytesReadTotal += bytesRead
		}
		return NewArrayFromComponenterSlice(array), bytesReadTotal, nil
	case ':': // integer
		var asInt int
		asInt, bytesRead, err = readFieldAsInt(reader, buffer)
		if err != nil {
			return
		}
		bytesReadTotal += bytesRead
		return NewIntFromInt(asInt), bytesReadTotal, nil
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
		if -1 == strLen {
			return NewNullString(), bytesReadTotal, nil
		}
		var theString string
		theString, bytesRead, err = readFieldAsBulkString(reader, buffer, strLen)
		bytesReadTotal += bytesRead
		return NewBulkStringFromString(theString), bytesReadTotal, nil
	case '+': // simple string
		var theString string
		theString, bytesRead, err = readFieldAsSimpleString(reader, buffer)
		if err != nil {
			return nil, bytesReadTotal, fmt.Errorf("unable to read Simple String at offset: %d", bytesReadTotal)
		}
		bytesReadTotal += bytesRead
		return NewSimpleStringFromString(theString), bytesReadTotal, nil
	case '-': // Error message
		var theString string
		theString, bytesRead, err = readFieldAsSimpleString(reader, buffer)
		if err != nil {
			return nil, bytesReadTotal, fmt.Errorf("unable to read Error Message at offset: %d", bytesReadTotal)
		}
		bytesReadTotal += bytesRead
		return NewErrorFromString(theString), bytesReadTotal, nil
	default:
		return nil, bytesReadTotal, fmt.Errorf("unrecognized field type: '%c'", fieldType[0])
	}
}

var RecordEndMarker = []byte{'\r', '\n'}

func readFieldAsInt(reader io.Reader, buffer []byte) (fieldValue int, bytesReadTotal int, err error) {
	var intStr string
	intStr, bytesReadTotal, err = readFieldAsSimpleString(reader, buffer)
	if err != nil {
		return
	}
	fieldValue, err = strconv.Atoi(intStr)
	if err != nil {
		return
	}
	return
}

func readFieldAsSimpleString(reader io.Reader, buffer []byte) (fieldValue string, bytesReadTotal int, err error) {
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

func readFieldAsBulkString(reader io.Reader, buffer []byte, stringLen int) (fieldValue string, bytesReadTotal int, err error) {
	var bytesRead int
	buffer = buffer[0:stringLen]
	bytesRead, err = reader.Read(buffer)
	if err != nil {
		return
	}
	bytesReadTotal += bytesRead
	if stringLen != bytesRead {
		err = fmt.Errorf("error reading bulk string, did not read enough bytes")
		return
	}
	fieldValue = string(buffer)

	// read OK, consume the \r\n
	buffer = buffer[0:len(RecordSeparator)]
	bytesRead, err = reader.Read(buffer)
	if err != nil {
		return
	}
	bytesReadTotal += bytesRead
	if len(RecordSeparator) != bytesRead {
		err = fmt.Errorf("error reading bulk string, unable to read the record separator")
		return
	}
	return
}

func resetBuffer(buffer []byte) []byte {
	return buffer[0:cap(buffer)]
}

func ComponentToStream(writer io.Writer, componenter Componenter) (totalBytesWritten int, err error) {
	var bytesWritten int
	switch componentType := componenter.(type) {
	case *Array:
		bytesWritten, err = fmt.Fprintf(writer, "*%d%s", len(*componentType), RecordSeparator)
		if err != nil {
			return
		}
		totalBytesWritten += bytesWritten
		for _, value := range *componentType {
			bytesWritten, err = ComponentToStream(writer, value)
			if err != nil {
				return
			}
			totalBytesWritten += bytesWritten
		}
	case *Int:
		totalBytesWritten, err = fmt.Fprintf(writer, ":%d%s", componentType.Int(), RecordSeparator)
	case *BulkString:
		s := componentType.String()
		totalBytesWritten, err = fmt.Fprintf(writer, "$%d%s%s%s", len(s), RecordSeparator, s, RecordSeparator)
	case *SimpleString:
		s := componentType.String()
		totalBytesWritten, err = fmt.Fprintf(writer, "$%d%s%s%s", len(s), RecordSeparator, s, RecordSeparator)
	case *Null:
		totalBytesWritten, err = fmt.Fprintf(writer, "*-1%s", RecordSeparator)
	case *NullString:
		totalBytesWritten, err = fmt.Fprintf(writer, "$-1%s", RecordSeparator)
	case *ErrorComp:
		s := componentType.String()
		totalBytesWritten, err = fmt.Fprintf(writer, "-%s%s", s, RecordSeparator)
	default:
		err = fmt.Errorf("unrecognized component type for object '%v'", componenter)
	}
	return
}
