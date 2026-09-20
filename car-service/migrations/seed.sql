-- Local development seed: cars
-- Lower ID => older created_at

INSERT INTO cars (name, price_cents, is_available, created_at, updated_at)
VALUES
  ('Toyota Prius',         2800000, true, NOW() - INTERVAL '20 days', NOW() - INTERVAL '20 days'),
  ('Honda Civic',          2600000, true, NOW() - INTERVAL '19 days', NOW() - INTERVAL '19 days'),
  ('Tesla Model 3',        4200000, true, NOW() - INTERVAL '18 days', NOW() - INTERVAL '18 days'),
  ('BMW 3 Series',         5100000, true, NOW() - INTERVAL '17 days', NOW() - INTERVAL '17 days'),
  ('Audi A4',              5000000, true, NOW() - INTERVAL '16 days', NOW() - INTERVAL '16 days'),
  ('Mercedes-Benz C-Class',5300000, true, NOW() - INTERVAL '15 days', NOW() - INTERVAL '15 days'),
  ('Volkswagen Golf',      2400000, true, NOW() - INTERVAL '14 days', NOW() - INTERVAL '14 days'),
  ('Ford Focus',           2300000, true, NOW() - INTERVAL '13 days', NOW() - INTERVAL '13 days'),
  ('Hyundai Elantra',      2200000, true, NOW() - INTERVAL '12 days', NOW() - INTERVAL '12 days'),
  ('Kia Forte',            2100000, true, NOW() - INTERVAL '11 days', NOW() - INTERVAL '11 days'),
  ('Mazda 3',              2500000, true, NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days'),
  ('Subaru Impreza',       2450000, true, NOW() - INTERVAL '9 days',  NOW() - INTERVAL '9 days'),
  ('Nissan Altima',        2700000, true, NOW() - INTERVAL '8 days',  NOW() - INTERVAL '8 days'),
  ('Chevrolet Malibu',     2650000, true, NOW() - INTERVAL '7 days',  NOW() - INTERVAL '7 days'),
  ('Peugeot 308',          2350000, true, NOW() - INTERVAL '6 days',  NOW() - INTERVAL '6 days'),
  ('Renault Megane',       2300000, true, NOW() - INTERVAL '5 days',  NOW() - INTERVAL '5 days'),
  ('Skoda Octavia',        2550000, true, NOW() - INTERVAL '4 days',  NOW() - INTERVAL '4 days'),
  ('Volvo S60',            4800000, true, NOW() - INTERVAL '3 days',  NOW() - INTERVAL '3 days'),
  ('Lexus IS',             5200000, true, NOW() - INTERVAL '2 days',  NOW() - INTERVAL '2 days'),
  ('Porsche 911',         12500000, true, NOW() - INTERVAL '1 day',   NOW() - INTERVAL '1 day')
ON CONFLICT (name) DO NOTHING;