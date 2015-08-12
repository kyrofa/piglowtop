/* Copyright (C) 2015 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify it under
 * the terms of the GNU General Public License as published by the Free Software
 * Foundation, either version 3 of the License, or (at your option) any later
 * version.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
 * FOR A PARTICULAR PURPOSE. See the GNU General Public License for more
 * details.
 *
 * You should have received a copy of the GNU General Public License along with
 * this program. If not, see <http://www.gnu.org/licenses/>.
 *
 * Author: Kyle Fazzari <kyle@canonical.com>
 */

package main

import (
	"flag"
	"github.com/c9s/goprocinfo/linux"
	"github.com/schoentoon/piglow"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// main is the entry point of this program. It accepts two command-line
// parameters:
//
// - brightness: Percentage of max LED brightness (between 0 and 1.0).
// - period: CPU poll period (in milliseconds).
func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	pollPeriodParameter := flag.Int("period", 200,
		"CPU poll period (in milliseconds)")
	brightnessParameter := flag.Float64("brightness", 0.02,
		"LED brightness (% of max brightness)")
	flag.Parse()

	if *brightnessParameter < 0 || *brightnessParameter > 1.0 {
		log.Fatalf("Brightness must be a value between 0 and 1.0 (got %v)",
			*brightnessParameter)
	}

	// Make sure we have a PiGlow to use. If we don't, provide a helpful error
	// message and exit, but wait several seconds before doing so in case systemd
	// respawns us. We don't want to hit its start limit.
	if !piglow.HasPiGlow() {
		log.Println("Unable to access PiGlow. Perhaps you need to hw-assign it?")
		time.Sleep(6 * time.Second)
		os.Exit(1)
	}

	// Initialize the CPU info so we can start out with a history
	previousStat := cpuStats()

	ticker := time.NewTicker(time.Duration(*pollPeriodParameter) *
		time.Millisecond)

	// Run this loop in a go routine so we can stop on demand.
	go func() {
		for _ = range ticker.C {
			currentStat := cpuStats()

			// Calculate previous idle and total ticks
			previousIdle := previousStat.Idle + previousStat.IOWait
			previousTotal := previousStat.User + previousStat.Nice +
				previousStat.System + previousStat.Idle +
				previousStat.IOWait + previousStat.IRQ +
				previousStat.SoftIRQ

			// Calculate current idle and total ticks
			currentIdle := currentStat.Idle + currentStat.IOWait
			currentTotal := currentStat.User + currentStat.Nice +
				currentStat.System + currentStat.Idle +
				currentStat.IOWait + currentStat.IRQ +
				currentStat.SoftIRQ

			// Calculating idle here since it's less typing. We'll invert it later.
			percentIdle := float64(currentIdle-previousIdle) /
				float64(currentTotal-previousTotal)

			displayUtilization(1.0-percentIdle, *brightnessParameter)

			previousStat = currentStat
		}
	}()

	<-signals         // Wait for request to stop
	ticker.Stop()     // We've been asked to stop
	piglow.ShutDown() // Turn off all LEDs
}

// cpuStats reads /proc/stat to obtain CPU utilization statistics.
func cpuStats() linux.CPUStat {
	stat, err := linux.ReadStat("/proc/stat")
	if err != nil {
		log.Fatalf("Unable to process /proc/stat: %s", err)
	}

	return stat.CPUStatAll
}

// displayUtilization takes a utilization percentage (between 0 and 1.0) and a
// brightness percentage (between 0 and 1.0) and displays them via the PiGlow's
// rings. 0% utilization -> no LEDs lit up. 50% utilization -> half the LEDs lit
// up, starting in the center. 100% utilization -> all LEDs lit up.
func displayUtilization(utilization float64, brightness float64) {
	brightnessByte := byte(math.Floor(brightness*255.0 + 0.5))

	// Using 6 here instead of 5 so no LEDs are lit for no utilization.
	lastRing := 6 - int(math.Floor(utilization*6.0+0.5))

	for ring := 0; ring <= 5; ring++ {
		if ring >= lastRing {
			piglow.Ring(byte(ring), brightnessByte)
		} else {
			piglow.Ring(byte(ring), 0x00)
		}
	}
}
