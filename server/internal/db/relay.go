package db

import "context"

type Relay struct {
	ID         int64  `db:"id"`
	Label      string `db:"label"`
	RelayIndex int64  `db:"relay_index"`
}

func CreateRelay(ctx context.Context, label string, index int64) int64 {
	res, err := DB.ExecContext(ctx, `INSERT INTO relays (label, relay_index) VALUES (?, ?)`, label, index)
	if err != nil {
		panic(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		panic(err)
	}
	return id
}

func GetRelayByIndex(ctx context.Context, index int64) (*Relay, error) {
	var relay Relay
	err := DB.GetContext(ctx, &relay, `SELECT * FROM relays WHERE relay_index = ?`, index)
	if err != nil {
		return nil, err
	}
	return &relay, nil
}

func ListRelays(ctx context.Context) *[]Relay {
	var relays []Relay
	if err := DB.SelectContext(ctx, &relays, `SELECT * FROM relays ORDER BY relay_index ASC`); err != nil {
		panic(err)
	}
	return &relays
}

func UpdateRelayLabel(ctx context.Context, index int64, label string) error {
	_, err := DB.ExecContext(ctx, `UPDATE relays SET label = ? WHERE relay_index = ?`, label, index)
	return err
}
