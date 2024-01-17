-- @package app1
-- +up
-- +begin
CREATE TABLE app1_a(a int);
-- +end

-- +down

-- +begin
DROP TABLE app1_a;
-- +end
