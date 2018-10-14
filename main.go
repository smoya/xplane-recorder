package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"os"

	"github.com/deet/simpleline"

	"github.com/smoya/xptokml/ui"

	"github.com/ornen/go-xplane"
	"github.com/ornen/go-xplane/messages"
	"github.com/pkg/errors"
	"github.com/twpayne/go-kml"

	"time"
)

var (
	now              time.Time
	foldersMap       map[string]*folder
	receiveCtx       context.Context
	receiveCtxCancel context.CancelFunc
)

type status struct {
	rpm              *messages.EngineRPMMessage
	control          *messages.FlightControlMessage
	gearsBrakes      *messages.GearsBrakesMessage
	gload            *messages.GLoadMessage
	latLonAlt        *messages.LatLonAltMessage
	pitchRollHeading *messages.PitchRollHeadingMessage
	speed            *messages.SpeedMessage
	trimFlapsBrakes  *messages.TrimFlapsBrakesMessage
	weather          *messages.WeatherMessage
	angular          *messages.AngularVelocitiesMessage
}

func (s *status) Complete() bool {
	return s.rpm != nil &&
		s.control != nil &&
		s.gearsBrakes != nil &&
		s.gload != nil &&
		s.latLonAlt != nil &&
		s.pitchRollHeading != nil &&
		s.speed != nil &&
		s.trimFlapsBrakes != nil &&
		s.weather != nil &&
		s.angular != nil
}

func (s *status) OnGround() bool {
	return s.latLonAlt.AltitudeAGL == 0
}

func main() {
	// TODO show address on UI
	x := xplane.New("127.0.0.1:49000", "127.0.0.1:49005")
	x.Connect()
	go x.Receive()

	startFunc := func() error {
		log.Println("Logging Start")
		now = time.Now()

		receiveCtx, receiveCtxCancel = context.WithCancel(context.Background())
		foldersMap = map[string]*folder{
			"flight": {
				name: kml.Name("Flight"),
				// todo style
				subfolder: &folder{
					name: kml.Name(fmt.Sprintf("FlightLog %s", now.Format(time.RFC3339))),
					// todo style
				},
			},
			"special": {
				name: kml.Name("Special placemarks"),
				// todo style
			},
			"data": {
				name: kml.Name("Flight data"),
				// todo style
			},
		}

		go readMessages(receiveCtx, x.Messages, foldersMap)

		return nil
	}

	stopFunc := func() error {
		log.Println("Logging Stop")
		receiveCtxCancel()
		return nil
	}

	saveFunc := func(flightName, filename string) error {
		log.Println("Saving kml")
		docName := fmt.Sprintf("Flightlog %s %s", flightName, now.Format(time.RFC3339))
		doc := createBaseDoc(docName)
		doc.Add(
			foldersMap["flight"].kml(),
			foldersMap["special"].kml(),
			foldersMap["data"].kml(),
		)

		f, err := os.Create(filename)
		if err != nil {
			return errors.Wrap(err, "error opening file")
		}
		defer f.Close()

		return kml.GxKML(doc).WriteIndent(f, "", "  ")
	}

	err := ui.Initialize(startFunc, stopFunc, saveFunc)
	if err != nil {
		panic(err.Error())
	}

}
func createBaseDoc(docName string) *kml.CompoundElement {
	return kml.Document(
		kml.Name(docName),
		kml.SharedStyleMap(
			"msn_ylw-pushpin",
			kml.Pair(kml.Key("normal"), kml.StyleURL("#sn_ylw-pushpin")),
			kml.Pair(kml.Key("highlight"), kml.StyleURL("#sh_ylw-pushpin")),
		),
		kml.SharedStyle(
			"sn_ylw-pushpin",
			kml.Pair(kml.Key("normal"), kml.StyleURL("#sn_ylw-pushpin")),
			kml.Pair(kml.Key("highlight"), kml.StyleURL("#sh_ylw-pushpin")),
		),
		kml.SharedStyleMap(
			"sn_ylw-pushpin",
			kml.IconStyle(
				kml.Scale(1.1),
				kml.Icon(kml.Href("http://maps.google.com/mapfiles/kml/pushpin/ylw-pushpin.png")),
				kml.HotSpot(kml.Vec2{X: 20, Y: 2, XUnits: "pixels", YUnits: "pixels"}),
				kml.LineStyle(kml.Color(color.RGBA{A: 255, G: 255}), kml.Width(2)),
				kml.LineStyle(kml.Color(color.RGBA{R: 127, B: 255})),
			),
		),
	)
}

func readMessages(ctx context.Context, chanMessages chan xplane.Message, foldersMap map[string]*folder) {
	var s status

	for {
		select {
		case m := <-chanMessages:
			switch m.(type) {
			case messages.LatLonAltMessage:
				mm := m.(messages.LatLonAltMessage)

				trackFlightPlacemark(&mm, &s, foldersMap)
				s.latLonAlt = &mm // should be placed after track

				trackFlightDataPlacemark(&s, foldersMap)

				// TODO track special placemarks

			case messages.EngineRPMMessage:
				mm := m.(messages.EngineRPMMessage)
				s.rpm = &mm
			case messages.FlightControlMessage:
				mm := m.(messages.FlightControlMessage)
				s.control = &mm
			case messages.GearsBrakesMessage:
				mm := m.(messages.GearsBrakesMessage)
				s.gearsBrakes = &mm
			case messages.PitchRollHeadingMessage:
				mm := m.(messages.PitchRollHeadingMessage)
				s.pitchRollHeading = &mm
			case messages.SpeedMessage:
				mm := m.(messages.SpeedMessage)
				s.speed = &mm
			case messages.TrimFlapsBrakesMessage:
				mm := m.(messages.TrimFlapsBrakesMessage)
				s.trimFlapsBrakes = &mm
			case messages.AngularVelocitiesMessage:
				mm := m.(messages.AngularVelocitiesMessage)
				s.angular = &mm

				//case messages.GLoadMessage:
				//	mm := m.(messages.GLoadMessage)
				//	s.gload = &mm
				//case messages.WeatherMessage:
				//	mm := m.(messages.WeatherMessage)
				//	s.weather = &mm
			}
		case <-ctx.Done():
			return
		}

	}
}

func trackFlightPlacemark(mm *messages.LatLonAltMessage, s *status, foldersMap map[string]*folder) {
	var flightPlacemark *placemark
	onGround := mm.AltitudeAGL <= 0

	if s.latLonAlt == nil {
		// TODO new placemark
		flightPlacemark = newFlightPlacemark(onGround)
		// TODO Check if this works
		foldersMap["flight"].subfolder.addPlacemark(flightPlacemark)
	} else {

		if s.OnGround() != onGround {
			// TODO new placemark
			flightPlacemark = newFlightPlacemark(onGround)
			// TODO Check if this works
			foldersMap["flight"].subfolder.addPlacemark(flightPlacemark)
		} else {
			p, err := foldersMap["flight"].subfolder.latestPlacemark()
			if err != nil {
				log.Println(err.Error())
				return
			}

			flightPlacemark = p
		}
	}

	c := kml.Coordinate{
		Lon: mm.Longitude,
		Lat: mm.Latitude,
	}
	if !onGround {
		c.Alt = mm.AltitudeMSL // TODO CHECK IF IT SHOULD BE AGL
	}
	flightPlacemark.lineString.addCoordinate(c)
}

func trackFlightDataPlacemark(s *status, foldersMap map[string]*folder) {
	if !s.Complete() {
		return
	}

	c := kml.Coordinate{
		Lon: s.latLonAlt.Longitude,
		Lat: s.latLonAlt.Latitude,
		Alt: s.latLonAlt.AltitudeMSL, // TODO CHECK IF IT SHOULD BE AGL
	}

	latest, _ := foldersMap["data"].latestPlacemark()
	if latest != nil {

		// Simplify coordinates line using RDP algorithm.
		// See https://en.wikipedia.org/wiki/Ramer%E2%80%93Douglas%E2%80%93Peucker_algorithm.
		previousCoordinate := latest.lineString.coordinates[len(latest.lineString.coordinates)-1]
		points := []simpleline.Point{
			&simpleline.Point3d{
				X: previousCoordinate.Lat,
				Y: previousCoordinate.Lon,
				Z: previousCoordinate.Alt,
			},
			&simpleline.Point3d{
				X: c.Lat,
				Y: c.Lon,
				Z: c.Alt,
			},
		}

		results, err := simpleline.RDP(points, 5, simpleline.Euclidean, true)
		if err != nil {
			// LOG
			// continue (discard simplification)
			log.Println("error simplifying coordinates, skipping...")
		} else {
			if len(results) < len(points) {
				// Simplification OK, so we don't add the new coordinate
				return
			}
		}
	}

	p := newFlightDataPlacemark(
		c,
		s.pitchRollHeading.HeadingMagnetic,
		s.pitchRollHeading.HeadingTrue,
		s.speed.IndicatedSpeed,
		s.speed.GroundSpeed,
		s.speed.TrueAirspeed,
		s.latLonAlt.AltitudeMSL,
		s.latLonAlt.AltitudeAGL,
		s.angular.Y, // TODO check if this is vertical speed
	)

	foldersMap["data"].addPlacemark(p)
}

type lineString struct {
	extrude      *kml.SimpleElement
	tessellate   *kml.SimpleElement
	altitude     *kml.SimpleElement
	altitudeMode *kml.SimpleElement
	coordinates  []kml.Coordinate
}

func (l *lineString) addCoordinate(c kml.Coordinate) {
	l.coordinates = append(l.coordinates, c)
}

func (l *lineString) kml() *kml.CompoundElement {
	ls := kml.LineString(
		l.extrude,
		l.tessellate,
	)

	if l.altitude != nil {
		ls.Add(l.altitude)
	}

	if l.altitudeMode != nil {
		ls.Add(l.altitudeMode)
	}

	ls.Add(kml.Coordinates(l.coordinates...))

	return ls
}

type placemark struct {
	name        *kml.SimpleElement
	description *kml.SimpleElement
	styleURL    *kml.SimpleElement
	lineString  *lineString
	point       *kml.CompoundElement
}

func (p *placemark) kml() *kml.CompoundElement {
	pm := kml.Placemark(
		p.name,
	)

	if p.description != nil {
		pm.Add(p.description)
	}

	if p.styleURL != nil {
		pm.Add(p.styleURL)
	}

	if p.lineString != nil {
		pm.Add(p.lineString.kml())
	}

	if p.point != nil {
		pm.Add(p.point)
	}

	return pm
}

type folder struct {
	name       *kml.SimpleElement
	style      *kml.CompoundElement
	placemarks []*placemark
	subfolder  *folder
}

func (f *folder) kml() *kml.CompoundElement {
	fl := kml.Folder(
		f.name,
		f.style,
	)

	for _, p := range f.placemarks {
		fl.Add(p.kml())
	}

	if f.subfolder != nil {
		fl.Add(f.subfolder.kml())
	}

	return fl
}

func (f *folder) latestPlacemark() (*placemark, error) {
	if len(f.placemarks) == 0 {
		return nil, errors.New("no placemarks")
	}

	return f.placemarks[len(f.placemarks)-1], nil
}

func (f *folder) addPlacemark(p *placemark) {
	f.placemarks = append(f.placemarks, p)
}

func newFlightPlacemark(onGround bool) *placemark {
	p := &placemark{
		styleURL: kml.StyleURL("#msn_ylw-pushpin"),
	}
	l := lineString{
		extrude:    kml.Extrude(true),
		tessellate: kml.Tessellate(true),
	}

	if onGround {
		p.name = kml.Name("On Ground")
		l.altitude = kml.Altitude(1)
	} else {
		p.name = kml.Name("In the air")
		l.altitudeMode = kml.AltitudeMode("absolute")
	}

	p.lineString = &l

	return p
}

func newFlightDataPlacemark(c kml.Coordinate, hdg, trueHDG, ias, gs, tas, alt, altAGL, vs float64) *placemark {
	desc := fmt.Sprintf(
		`<![CDATA[Time/Date: %s<br>Hdg: %6.2f<br>True Hdg: %6.2f<br>IAS: %6.2f<br>GS: %6.2f<br>TAS: %6.2f<br>Alt: %6.2f<br>Alt AGL: %6.2f<br>VS: %6.2f<br><br>Thank you for using the BlackBox flightlogger!<br>Check <a href="http://www.utr-online.com">the website</a> for updates frequently!]]>`,
		time.Now().String(),
		hdg,
		trueHDG,
		ias,
		gs,
		tas,
		alt,
		altAGL,
		vs,
	)

	return &placemark{
		description: kml.Description(desc),
		point:       kml.Point(kml.Coordinates(c)),
	}
}

func newSpecialPlacemark(name string, c kml.Coordinate, hdg, trueHDG, ias, gs, tas, alt, altAGL, vs float64) *placemark {
	p := newFlightDataPlacemark(c, hdg, trueHDG, ias, gs, tas, alt, altAGL, vs)
	p.name = kml.Name(name)

	return p
}
