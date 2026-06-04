package service

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
)

const (
	indexMagic       = "R26IVF1\n"
	vectorDims       = 14
	indexRecordSize  = vectorDims*2 + 1
	baseProbeCount   = 4
	repairProbeCount = 12
	topK             = 5
)

type VectorIndex struct {
	partitions [16]indexPartition
}

type indexPartition struct {
	clusters []indexCluster
	data     []byte
}

type indexCluster struct {
	center [vectorDims]int16
	offset uint32
	count  uint32
}

type centerCandidate struct {
	index int
	dist  int64
}

type nearestSet struct {
	dist  [topK]int64
	label [topK]byte
}

func LoadVectorIndex(path string) (*VectorIndex, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(blob) < len(indexMagic) || string(blob[:len(indexMagic)]) != indexMagic {
		return nil, errors.New("invalid vector index")
	}

	cursor := len(indexMagic)
	index := &VectorIndex{}
	for part := 0; part < len(index.partitions); part++ {
		clusterCount, ok := readUint32(blob, &cursor)
		if !ok {
			return nil, errors.New("truncated vector index")
		}
		dataLen, ok := readUint32(blob, &cursor)
		if !ok {
			return nil, errors.New("truncated vector index")
		}

		clusters := make([]indexCluster, int(clusterCount))
		for i := range clusters {
			for dim := 0; dim < vectorDims; dim++ {
				value, ok := readInt16(blob, &cursor)
				if !ok {
					return nil, errors.New("truncated vector index")
				}
				clusters[i].center[dim] = value
			}
			offset, ok := readUint32(blob, &cursor)
			if !ok {
				return nil, errors.New("truncated vector index")
			}
			count, ok := readUint32(blob, &cursor)
			if !ok {
				return nil, errors.New("truncated vector index")
			}
			clusters[i].offset = offset
			clusters[i].count = count
		}

		next := cursor + int(dataLen)
		if next < cursor || next > len(blob) {
			return nil, errors.New("truncated vector index")
		}
		index.partitions[part] = indexPartition{
			clusters: clusters,
			data:     blob[cursor:next],
		}
		cursor = next
	}

	return index, nil
}

func (idx *VectorIndex) FraudCount(query [vectorDims]int16) int {
	partition := idx.partitions[partitionVector(query)]
	if len(partition.clusters) == 0 {
		return 0
	}

	var candidates [repairProbeCount]centerCandidate
	for i := range candidates {
		candidates[i].dist = math.MaxInt64
	}
	for i := range partition.clusters {
		dist := centerDistance(query, partition.clusters[i].center)
		if dist >= candidates[repairProbeCount-1].dist {
			continue
		}
		pos := repairProbeCount - 1
		for pos > 0 && dist < candidates[pos-1].dist {
			candidates[pos] = candidates[pos-1]
			pos--
		}
		candidates[pos] = centerCandidate{index: i, dist: dist}
	}

	nearest := newNearestSet()
	scanCandidateClusters(&partition, query, &nearest, candidates[:baseProbeCount])
	fraudCount := nearest.fraudCount()
	if fraudCount == 2 || fraudCount == 3 {
		scanCandidateClusters(&partition, query, &nearest, candidates[baseProbeCount:repairProbeCount])
		fraudCount = nearest.fraudCount()
	}
	return fraudCount
}

func scanCandidateClusters(partition *indexPartition, query [vectorDims]int16, nearest *nearestSet, candidates []centerCandidate) {
	for _, candidate := range candidates {
		if candidate.dist == math.MaxInt64 {
			continue
		}
		cluster := partition.clusters[candidate.index]
		start := int(cluster.offset)
		end := start + int(cluster.count)*indexRecordSize
		if start < 0 || end > len(partition.data) {
			continue
		}
		for offset := start; offset < end; offset += indexRecordSize {
			nearest.add(recordDistance(query, partition.data[offset:offset+indexRecordSize]), partition.data[offset+vectorDims*2])
		}
	}
}

func newNearestSet() nearestSet {
	return nearestSet{
		dist: [topK]int64{math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64},
	}
}

func (set *nearestSet) add(dist int64, label byte) {
	if dist >= set.dist[topK-1] {
		return
	}
	pos := topK - 1
	for pos > 0 && dist < set.dist[pos-1] {
		set.dist[pos] = set.dist[pos-1]
		set.label[pos] = set.label[pos-1]
		pos--
	}
	set.dist[pos] = dist
	set.label[pos] = label
}

func (set *nearestSet) fraudCount() int {
	count := 0
	for _, label := range set.label {
		count += int(label)
	}
	return count
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

func centerDistance(query, center [vectorDims]int16) int64 {
	var sum int64
	for i := 0; i < vectorDims; i++ {
		diff := int64(query[i]) - int64(center[i])
		sum += diff * diff
	}
	return sum
}

func recordDistance(query [vectorDims]int16, record []byte) int64 {
	var sum int64
	for i := 0; i < vectorDims; i++ {
		value := int16(binary.LittleEndian.Uint16(record[i*2:]))
		diff := int64(query[i]) - int64(value)
		sum += diff * diff
	}
	return sum
}

func readUint32(blob []byte, cursor *int) (uint32, bool) {
	next := *cursor + 4
	if next > len(blob) {
		return 0, false
	}
	value := binary.LittleEndian.Uint32(blob[*cursor:next])
	*cursor = next
	return value, true
}

func readInt16(blob []byte, cursor *int) (int16, bool) {
	next := *cursor + 2
	if next > len(blob) {
		return 0, false
	}
	value := int16(binary.LittleEndian.Uint16(blob[*cursor:next]))
	*cursor = next
	return value, true
}
