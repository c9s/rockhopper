package rockhopper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_previewSQL(t *testing.T) {
	longText := `CREATE TABLE app1_a
	(
		gid            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

		id             BIGINT UNSIGNED,
		order_id       BIGINT UNSIGNED NOT NULL,
		exchange       VARCHAR(24) NOT NULL DEFAULT '',
		symbol         VARCHAR(20) NOT NULL,
		price          DECIMAL(16, 8) UNSIGNED NOT NULL,
		quantity       DECIMAL(16, 8) UNSIGNED NOT NULL,
		quote_quantity DECIMAL(16, 8) UNSIGNED NOT NULL,
		fee            DECIMAL(16, 8) UNSIGNED NOT NULL,
		fee_currency   VARCHAR(10) NOT NULL,
		is_buyer       BOOLEAN     NOT NULL DEFAULT FALSE,
		is_maker       BOOLEAN     NOT NULL DEFAULT FALSE,
		side           VARCHAR(4)  NOT NULL DEFAULT '',
		traded_at      DATETIME(3) NOT NULL,

		is_margin      BOOLEAN     NOT NULL DEFAULT FALSE,
		is_isolated    BOOLEAN     NOT NULL DEFAULT FALSE,

		strategy       VARCHAR(32) NULL,
		pnl            DECIMAL NULL,

		PRIMARY KEY (gid),
		UNIQUE KEY id (exchange, symbol, side, id)
	);`

	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "long text",
			args: args{s: longText},
			want: 60,
		},
		{
			name: "padded",
			args: args{s: "CREATE TABLE a()"},
			want: 60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, len(previewSQL(tt.args.s)), "previewSQL(%v)", tt.args.s)
		})
	}
}
