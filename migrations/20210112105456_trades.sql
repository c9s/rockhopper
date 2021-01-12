-- +up
-- +begin
create table a (id int);
-- +end

-- +down

-- +begin
drop table a;
-- +end
