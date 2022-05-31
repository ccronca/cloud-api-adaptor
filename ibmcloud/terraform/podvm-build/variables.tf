variable "ibmcloud_api_key" {
    sensitive = true
}

variable "cluster_name" {
    description = "Prefix for the Kubernetes control plane and worker instances. Must be provided if worker_ip and bastion_ip are not provided"
    default = null
}

variable "ibmcloud_user_id" {
    description = "User ID that owns the provided IBM Cloud API key"
}

variable "region_name" {
    default = "jp-tok"
}

variable "vpc_name" {
    default = "tok-vpc"
}

variable "primary_subnet_name" {
    default = "tok-primary-subnet"
}

variable "cos_service_instance_name" {
    description = "Name of the COS service instance. Must be provided if cos_service_instance_id is not provided"
    default = null
}

variable "cos_service_instance_id" {
    description = "ID of the COS service instance. Must be provided if cos_service_instance_name is not provided"
    default = null
}

variable "cos_bucket_name" {
    description = "Name of the COS bucket"
}

variable "use_ibmcloud_test" {
    type = bool
    default = false
}

variable "worker_ip" {
    description = "Internal ipv4 address assigned to the worker instance. Must be provided if cluster_name is not provided"
    default = null
}

variable "bastion_ip" {
    description = "Floating ipv4 address assigned to the worker instance. Must be provided if cluster_name is not provided"
    default = null
}

variable "podvm_image_name" {
    description = "Name to give the VPC custom image. If not provided, the image tag is based on it git commit hash of the cloud-api-adaptor code base being built on the worker instance"
    default = null
}

variable "ansible_dir" {
    description = "Subdirectory for Ansible playbook, inventory and vars files"
    default = "./ansible"
}