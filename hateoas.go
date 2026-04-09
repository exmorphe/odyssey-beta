package main

import "encoding/json"

// Link is a HATEOAS link from a server response.
type Link struct {
	Href   string `json:"href"`
	Method string `json:"method"`
	Title  string `json:"title"`
}

// ServerResponse is a parsed server response with typed links and raw fields.
type ServerResponse struct {
	Type  string
	Links map[string]Link
	raw   map[string]any
}

// Field returns a raw field value by name.
func (r *ServerResponse) Field(name string) (any, bool) {
	v, ok := r.raw[name]
	return v, ok
}

// ParseResponse parses a JSON response body into a ServerResponse,
// extracting _type and _links while preserving all raw fields.
func ParseResponse(data []byte) (*ServerResponse, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	resp := &ServerResponse{raw: raw}

	if t, ok := raw["_type"].(string); ok {
		resp.Type = t
	}

	resp.Links = make(map[string]Link)
	if linksRaw, ok := raw["_links"]; ok {
		linksBytes, err := json.Marshal(linksRaw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(linksBytes, &resp.Links); err != nil {
			return nil, err
		}
	}

	return resp, nil
}
