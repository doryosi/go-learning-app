output "vpc_id" {
  description = "ID of the learning-lab VPC."
  value       = aws_vpc.this.id
}

output "availability_zones" {
  description = "Availability Zones selected for the network."
  value       = local.azs
}

output "public_subnet_ids" {
  description = "Public subnet IDs keyed by Availability Zone."
  value       = { for az, subnet in aws_subnet.public : az => subnet.id }
}

output "private_subnet_ids" {
  description = "Private subnet IDs keyed by Availability Zone."
  value       = { for az, subnet in aws_subnet.private : az => subnet.id }
}

output "public_route_table_id" {
  description = "Route table used by all public subnets."
  value       = aws_route_table.public.id
}

output "private_route_table_ids" {
  description = "Private route table IDs keyed by Availability Zone."
  value       = { for az, table in aws_route_table.private : az => table.id }
}

output "nat_gateway_ids" {
  description = "NAT gateway IDs. Empty when nat_gateway_mode is none."
  value       = { for az, gateway in aws_nat_gateway.this : az => gateway.id }
}

output "alb_security_group_id" {
  description = "Security group for the future public Application Load Balancer."
  value       = aws_security_group.alb.id
}

output "app_security_group_id" {
  description = "Security group for private application compute."
  value       = aws_security_group.app.id
}
