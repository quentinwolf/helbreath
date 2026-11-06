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
	if got := e.worldStateFor("icebound"); got[len(got)-4:] != "snow" {
		t.Errorf("icebound world state = %q, want snow all year", got)
	}
	if e.weatherOn("icebound") != world.WeatherSnow {
		t.Errorf("weatherOn(icebound) should be snow")
	}
}
