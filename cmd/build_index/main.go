package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

const (
	indexMagic      = "R26IVF1\n"
	vectorDims      = 14
	vectorScale     = 10000
	clusterCount    = 512
	kmeansRounds    = 5
	kmeansSampleCap = 50000
	recordSize      = vectorDims*2 + 1
)

type refEntry struct {
	Vector []float64 `json:"vector"`
	Label  string    `json:"label"`
}

type point struct {
	vector [vectorDims]int16
	label  byte
}

type cluster struct {
	center [vectorDims]int16
	points []point
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: build_index <references.json.gz> <index.bin>")
		os.Exit(2)
	}

	partitions, err := loadReferences(os.Args[1])
	if err != nil {
		panic(err)
	}
	clusters := buildIndex(partitions)
	if err := writeIndex(os.Args[2], clusters); err != nil {
		panic(err)
	}
}

func loadReferences(path string) ([16][]point, error) {
	file, err := os.Open(path)
	if err != nil {
		return [16][]point{}, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return [16][]point{}, err
	}
	defer gz.Close()

	decoder := json.NewDecoder(gz)
	if _, err := decoder.Token(); err != nil {
		return [16][]point{}, err
	}

	var partitions [16][]point
	for decoder.More() {
		var entry refEntry
		if err := decoder.Decode(&entry); err != nil {
			return [16][]point{}, err
		}
		var p point
		for i, value := range entry.Vector {
			p.vector[i] = int16(math.Round(value * vectorScale))
		}
		if entry.Label == "fraud" {
			p.label = 1
		}
		part := partitionVector(p.vector)
		partitions[part] = append(partitions[part], p)
	}
	return partitions, nil
}

func buildIndex(partitions [16][]point) [16][]cluster {
	var index [16][]cluster
	for part, points := range partitions {
		if len(points) == 0 {
			continue
		}
		k := clusterCount
		if len(points) < k {
			k = len(points)
		}

		centers := make([][vectorDims]int16, k)
		for i := range centers {
			centers[i] = points[(i*len(points)+len(points)/(2*k))/k].vector
		}

		sampleStep := 1
		if len(points) > kmeansSampleCap {
			sampleStep = len(points) / kmeansSampleCap
		}
		for round := 0; round < kmeansRounds; round++ {
			sums := make([][vectorDims]int64, k)
			counts := make([]int64, k)
			for i := 0; i < len(points); i += sampleStep {
				nearest := nearestCenter(points[i].vector, centers)
				counts[nearest]++
				for dim := 0; dim < vectorDims; dim++ {
					sums[nearest][dim] += int64(points[i].vector[dim])
				}
			}
			for i := range centers {
				if counts[i] == 0 {
					continue
				}
				for dim := 0; dim < vectorDims; dim++ {
					centers[i][dim] = int16(sums[i][dim] / counts[i])
				}
			}
		}

		clusters := make([]cluster, len(centers))
		for i := range centers {
			clusters[i].center = centers[i]
		}

		counts := make([]int, len(centers))
		assignments := make([]int, len(points))
		for i := range points {
			nearest := nearestCenter(points[i].vector, centers)
			assignments[i] = nearest
			counts[nearest]++
		}
		for i, count := range counts {
			clusters[i].points = make([]point, 0, count)
		}
		for i, nearest := range assignments {
			clusters[nearest].points = append(clusters[nearest].points, points[i])
		}
		index[part] = clusters
	}
	return index
}

func writeIndex(path string, index [16][]cluster) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write([]byte(indexMagic)); err != nil {
		return err
	}
	for _, clusters := range index {
		if err := writeUint32(file, uint32(len(clusters))); err != nil {
			return err
		}
		dataLen := 0
		for _, c := range clusters {
			dataLen += len(c.points) * recordSize
		}
		if err := writeUint32(file, uint32(dataLen)); err != nil {
			return err
		}

		offset := uint32(0)
		for _, c := range clusters {
			for _, value := range c.center {
				if err := writeInt16(file, value); err != nil {
					return err
				}
			}
			if err := writeUint32(file, offset); err != nil {
				return err
			}
			if err := writeUint32(file, uint32(len(c.points))); err != nil {
				return err
			}
			offset += uint32(len(c.points) * recordSize)
		}

		buffer := make([]byte, recordSize)
		for _, c := range clusters {
			for _, p := range c.points {
				for dim, value := range p.vector {
					binary.LittleEndian.PutUint16(buffer[dim*2:], uint16(value))
				}
				buffer[vectorDims*2] = p.label
				if _, err := file.Write(buffer); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func nearestCenter(vector [vectorDims]int16, centers [][vectorDims]int16) int {
	best := 0
	bestDist := int64(math.MaxInt64)
	for i, center := range centers {
		dist := distance(vector, center)
		if dist < bestDist {
			best = i
			bestDist = dist
		}
	}
	return best
}

func distance(a, b [vectorDims]int16) int64 {
	var sum int64
	for i := 0; i < vectorDims; i++ {
		diff := int64(a[i]) - int64(b[i])
		sum += diff * diff
	}
	return sum
}

func partitionVector(vector [vectorDims]int16) int {
	tag := 0
	if vector[10] > 0 {
		tag |= 8
	}
	if vector[9] > 0 {
		tag |= 4
	}
	if vector[11] > 0 {
		tag |= 2
	}
	if vector[5] >= 0 {
		tag |= 1
	}
	return tag
}

func writeUint32(file *os.File, value uint32) error {
	var buffer [4]byte
	binary.LittleEndian.PutUint32(buffer[:], value)
	_, err := file.Write(buffer[:])
	return err
}

func writeInt16(file *os.File, value int16) error {
	var buffer [2]byte
	binary.LittleEndian.PutUint16(buffer[:], uint16(value))
	_, err := file.Write(buffer[:])
	return err
}
