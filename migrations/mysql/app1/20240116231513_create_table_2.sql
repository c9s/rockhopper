-- @package app1
-- +up
-- +begin
CREATE TABLE app1_b(b int);
-- +end

-- +down

-- +begin
DROP TABLE app1_b;
-- +end
