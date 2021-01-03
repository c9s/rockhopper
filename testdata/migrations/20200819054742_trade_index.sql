-- +up
-- +begin
CREATE INDEX trades_symbol ON trades(symbol);
-- +end
-- +begin
CREATE INDEX trades_symbol_fee_currency ON trades(symbol, fee_currency, traded_at);
-- +end
-- +begin
CREATE INDEX trades_traded_at_symbol ON trades(traded_at, symbol);
-- +end

-- +down
-- +begin
DROP INDEX trades_symbol ON trades;
-- +end
-- +begin
DROP INDEX trades_symbol_fee_currency ON trades;
-- +end
-- +begin
DROP INDEX trades_traded_at_symbol ON trades;
-- +end
