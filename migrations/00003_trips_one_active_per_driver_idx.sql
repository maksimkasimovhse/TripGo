-- +goose Up
CREATE UNIQUE INDEX trips_one_active_per_driver_idx ON trips(driver_id) 
WHERE status = 'active';

-- +goose Down
DROP INDEX trips_one_active_per_driver_idx;
