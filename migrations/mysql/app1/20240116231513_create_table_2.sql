-- @package app1
-- +up
-- +begin
CREATE TABLE b(b int);
-- +end

-- +down

-- +begin
DROP TABLE b;
-- +end
