### The Ansible inventory file
resource "local_file" "inventory" { 
    content = templatefile("inventory.tpl",
        {
            public_ip = hcloud_server.control_plane.*.ipv4_address,
        }
    )
    filename = "../Ansible/inventory.ini"
}