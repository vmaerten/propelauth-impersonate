package main

import (
	"bytes"

	"github.com/pelletier/go-toml/v2/unstable"
)

// editTOML sets the top-level key of data to the TOML value literal, or removes it when literal
// is nil. Only the bytes of the value change, or the line of the key, so comments and formatting
// stay; a new key goes after the last top-level one, before the first table.
func editTOML(data []byte, key string, literal *string) ([]byte, error) {
	var p unstable.Parser
	p.Reset(data)
	valueStart, valueEnd, keyStart, insertAt := -1, -1, -1, 0
	for p.NextExpression() {
		e := p.Expression()
		if e.Kind != unstable.KeyValue {
			// Tables, [[envs]] included, end the top-level keys.
			break
		}
		it := e.Key()
		it.Next()
		insertAt = lineEnd(data, int(e.Raw.Offset+e.Raw.Length))
		if string(it.Node().Data) == key && !it.Next() {
			v := e.Value().Raw
			valueStart, valueEnd, keyStart = int(v.Offset), int(v.Offset+v.Length), int(e.Raw.Offset)
		}
	}
	if err := p.Error(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	switch {
	case keyStart >= 0 && literal != nil:
		out.Write(data[:valueStart])
		out.WriteString(*literal)
		out.Write(data[valueEnd:])
	case keyStart >= 0:
		out.Write(data[:bytes.LastIndexByte(data[:keyStart], '\n')+1])
		out.Write(data[lineEnd(data, valueEnd):])
	case literal == nil:
		return data, nil
	default:
		out.Write(data[:insertAt])
		if insertAt > 0 && data[insertAt-1] != '\n' {
			out.WriteByte('\n')
		}
		out.WriteString(key + " = " + *literal + "\n")
		out.Write(data[insertAt:])
	}
	return out.Bytes(), nil
}

// lineEnd returns the offset just past the end of the line holding offset.
func lineEnd(data []byte, offset int) int {
	if i := bytes.IndexByte(data[offset:], '\n'); i >= 0 {
		return offset + i + 1
	}
	return len(data)
}
