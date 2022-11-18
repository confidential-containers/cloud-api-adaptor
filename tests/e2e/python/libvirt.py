"""
Copyright Confidential Containers Contributors
SPDX-License-Identifier: Apache-2.0

This module contain the pipeline implementation for Libvirt.
"""
import os
from subprocess import run
from subprocess import CalledProcessError
from tempfile import TemporaryDirectory
from base import get_project_dir
from base import PipelineABC
from base import step
from base import StepException

class LibvirtPipeline(PipelineABC):
    """
    Implement the pipeline for libvirt.
    """

    network = 'default'
    storage_pool = 'default'
    uri = 'qemu:///system'
    workdir = None

    def __init__(self):
        self.libvirt_dir = os.path.join(get_project_dir(), "libvirt")

    def get_required_commands(self):
        """
        Implement :func:`base.PipelineABC.get_required_commands`.
        """
        return ['docker', 'kcli', 'kubectl', 'virsh']

    def get_required_envvars(self):
        """
        Implement :func:`base.PipelineABC.get_required_envvars`.
        """
        return []

    def pipeline_setup(self):
        """
        Implement :func:`base.PipelineABC.pipeline_setup`.
        """
        self.workdir = TemporaryDirectory(prefix='caa-tests-libvirt-')
        return 0

    def pipeline_teardown(self):
        """
        Implement :func:`base.PipelineABC.pipeline_teardown`.
        """
        if self.workdir:
            self.workdir.cleanup()

    @step
    def step_vpc_create(self, log):
        """
        Implement :func:`base.PipelineABC.step_vpc_create`.
        """
        try:
            run('virsh -c %s net-info %s' % (self.uri, self.network),
                shell=True, check=True)
        except CalledProcessError:
            raise StepException("Libvirt network '%s' should be created beforehand" %
                self.network)
        try:
            res = run('virsh -c %s pool-info %s' % (self.uri, self.storage_pool),
                shell=True, check=True, capture_output=True, encoding='utf-8')
            # TODO grep the output for status running
            print(res.stdout)
        except CalledProcessError:
            #log.error(res.stdout)
            raise StepException("Libvirt storage pool '%s' should be created beforehand" %
                self.storage_pool)

    @step
    def step_cluster_create(self, log):
        """
        Implement :func:`base.PipelineABC.step_cluster_create`.
        """
        try:
            run('./kcli_cluster.sh create', shell=True, check=True,
                cwd=self.libvirt_dir)
            os.environ['KUBECONFIG'] = os.getenv('HOME') + \
                '/.kcli/clusters/peer-pods/auth/kubeconfig'
        except CalledProcessError:
            raise StepException("Failed to create the cluster")

    @step
    def step_podvm_create(self, log):
        """
        Implement :func:`base.PipelineABC.step_podvm_create`.
        """
        image_dir = os.path.join(self.libvirt_dir, "image")

        try:
            run('make docker-build', shell=True, check=True,
                cwd=image_dir)
        except CalledProcessError:
            raise StepException("Failed to create the podvm qcow2 file")

        try:
            run('make push', shell=True, check=True,
                cwd=image_dir)
        except CalledProcessError:
            raise StepException("Failed to create the podvm volume in libvirt")

    @step
    def step_install_peerpods(self, log):
        """
        Implement :func:`base.PipelineABC.step_install_peerpods`.
        """
        overlays_dir = os.path.join(get_project_dir(), "install", "overlays",
            "libvirt")
        id_rsa_file = os.path.join(overlays_dir, "id_rsa")
        id_rsa_pub_file = id_rsa_file + '.pub'
        authorized_keys_file = os.path.join(os.getenv('HOME'), '.ssh', 'authorized_keys')

        try:
            # TODO: revisit that algorithm:
            #        - ~/.ssh might not exist
            #        - ~/.ssh/authorized_keys might not exist and it will be created
            #          wrong permissions
            #        - user might pass a key pair
            #
            if os.path.exists(id_rsa_file):
                os.remove(id_rsa_file)
            if os.path.exists(id_rsa_pub_file):
                os.remove(id_rsa_pub_file)
            run('ssh-keygen -f %s -N ""' % id_rsa_file, shell=True, check=True)
            with open(authorized_keys_file, 'a+b') as auth_file:
                with open(id_rsa_pub_file, 'r+b') as pub_file:
                    auth_file.write(pub_file.read())

            os.environ['SSH_KEY_FILE'] = os.path.basename(id_rsa_file)
        except CalledProcessError:
            raise StepException("Failed to create a pair of SSH keys")

        try:
            # TODO: figure out the IP on runtime.
            os.environ['LIBVIRT_IP'] = "192.168.1.107"
            run('./install_operator.sh', shell=True, check=True,
                cwd=self.libvirt_dir)
        except CalledProcessError:
            raise StepException("Failed to install peer pods")
