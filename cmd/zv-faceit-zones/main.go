// Command zv-faceit-zones regenerates the CIS and LATAM rosters the Players
// section ships with, from the live FACEIT leaderboards:
//
//	FACEIT_API_KEY=... go run ./cmd/zv-faceit-zones -out internal/faceit/zones_default.json
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/rechedev9/cliphub/internal/faceit"
)

func main() {
	out := flag.String("out", "internal/faceit/zones_default.json", "seed document to write")
	limit := flag.Int("limit", faceit.SeedZoneLimit, "players per zone")
	flag.Parse()

	client, err := faceit.New(faceit.Options{APIKey: os.Getenv("FACEIT_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}
	store, err := faceit.NewSeedStore(*out)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	doc, err := store.Refresh(ctx, client, *limit)
	if err != nil {
		log.Fatal(err)
	}
	counts := map[string]int{}
	for _, player := range doc.Players {
		counts[player.Zone]++
	}
	for _, zone := range faceit.Zones() {
		fmt.Printf("%s: %d players\n", zone, counts[zone])
	}
}
