-- Remove redundant fields from servers table
-- node_ip: can be derived from port_allocations -> nodes.public_ip
-- pod_ip: internal cluster IP not needed by frontend
-- node_name: can be derived from port_allocations -> nodes.name

ALTER TABLE servers DROP COLUMN IF EXISTS node_ip;
ALTER TABLE servers DROP COLUMN IF EXISTS pod_ip;
ALTER TABLE servers DROP COLUMN IF EXISTS node_name;
