package game

import (
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/world"
)

func TestWorldStateIndoorsIsClearDay(t *testing.T) {
	e := &Engine{worldState: world.NewWorldState()}
	e.worldState.SetWeather(world.WeatherRain, time.Minute)
	e.CurrentWeather = world.WeatherFog

	if got := e.worldStateFor("Toh1"); got != "time:day|weather:clear" {
		t.Errorf("Toh1 world state = %q, want clear day", got)
	}
	if got := e.worldStateFor("middleland"); got[len(got)-4:] != "rain" {
		t.Errorf("middleland world state = %q, want rain", got)
	}
	if e.weatherOn("dglv2") != world.WeatherClear || e.weatherOn("aresden") != world.WeatherFog {
		t.Errorf("weatherOn should be clear indoors and the world weather outdoors")
	}
}
