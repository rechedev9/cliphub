package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type map3DIn struct {
	Map string `json:"map" jsonschema:"the map file name, such as de_mirage, as get_match reports it"`
}

type map3DOut struct {
	Map       string `json:"map"`
	Triangles int    `json:"triangles"`
	Textures  int    `json:"textures"`
	Lightmap  bool   `json:"lightmap" jsonschema:"the map's baked light from CS2"`
	Sun       bool   `json:"sun"`
	Fog       bool   `json:"fog"`
	Sky       bool   `json:"sky"`
	Post      bool   `json:"post" jsonschema:"the map's own exposure, tonemap curve and colour grading from CS2"`
	HowToOpen string `json:"how_to_open"`
}

func registerMap3DTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "map_3d_status",
		Description: "Check that Studio can draw a map as CS2 does in the tactical replay, in 3D and as the 2D view's Mapa real floor plan: its real surfaces, textures, baked light, sun, fog, sky and colour grading, read from the CS2 install on this PC. " +
			"The first call for a map reads it from CS2 and can take some seconds. The answer says how the user opens it, full screen included."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in map3DIn) (*mcp.CallToolResult, map3DOut, error) {
			if !mapNamePattern.MatchString(in.Map) {
				return nil, map3DOut{}, fmt.Errorf("map must be a map file name such as de_mirage, not %q", cleanName(in.Map))
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, map3DOut{}, toolError(err)
			}
			sum, err := c.GetMapSceneSummary(ctx, in.Map)
			if err != nil {
				return nil, map3DOut{}, toolError(err)
			}
			return nil, map3DOut{
				Map: sum.Map, Triangles: sum.Triangles, Textures: sum.Textures,
				Lightmap: sum.Lightmap, Sun: sum.Sun, Fog: sum.Fog, Sky: sum.Sky, Post: sum.Post,
				HowToOpen: "In Studio open the match and the Táctica tab. In the 3D view pick the Mapa real chip; in the 2D view pick Mapa real under the map for the floor plan seen from above. The Pantalla completa button or the P key fills the screen.",
			}, nil
		})
}
