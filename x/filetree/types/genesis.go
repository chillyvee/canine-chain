package types

import (
	"fmt"
)

// DefaultIndex is the default capability global index
const DefaultIndex uint64 = 1

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		FilesList: []Files{},
		// this line is used by starport scaffolding # genesis/types/default
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	// Check for duplicated index in files
	filesIndexMap := make(map[string]struct{})

	for _, elem := range gs.FilesList {
		index := string(FilesKey(elem.Address))
		if _, ok := filesIndexMap[index]; ok {
			return fmt.Errorf("duplicated index for files")
		}
		filesIndexMap[index] = struct{}{}
	}
	// this line is used by starport scaffolding # genesis/types/validate

	return gs.Params.Validate()
}
