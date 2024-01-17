-- @package app2
-- +up
-- +begin
CREATE TABLE app2_b(a int);
-- +end

-- +down

-- +begin
DROP TABLE app2_b;
-- +end
