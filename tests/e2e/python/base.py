"""
Copyright Confidential Containers Contributors
SPDX-License-Identifier: Apache-2.0

This module contains base classes and methods to implement end-to-end test pipelines.
"""
from abc import ABC
from abc import abstractmethod
from shutil import which
from pathlib import Path

import logging
import os
import sys

def get_project_dir():
    """
    Return the project absolute path.
    """
    # tests/e2e/python/../../
    return Path(__file__).absolute().parent.parent.parent.parent

class PipelineABC(ABC):
    """
    Represents a end-to-end test pipeline for peer pods.

    This is an abstract class that should have implementation for each cloud provider.
    """

    @abstractmethod
    def get_required_commands(self):
        """
        Return a list of required commands required to run the pipeline. Return an empty
        list if none is required.
        """
        raise NotImplementedError

    @abstractmethod
    def get_required_envvars(self):
        """
        Return a list of required environment variables to run the pipeline. Return an empty
        list if none is required.
        """
        raise NotImplementedError

    @abstractmethod
    def pipeline_setup(self):
        """
        Run immediately before the pipeline steps.
        """
        raise NotImplementedError

    @abstractmethod
    def pipeline_teardown(self):
        """
        Run after the last pipeline step regardless if the pipeline failed.
        """
        raise NotImplementedError

    @abstractmethod
    def step_vpc_create(self, log):
        """
        Step create VPC (Virtual Private Cloud).
        """
        raise NotImplementedError

    @abstractmethod
    def step_cluster_create(self, log):
        """
        Step create the Kubernetes cluster.
        """
        raise NotImplementedError

    @abstractmethod
    def step_podvm_create(self, log):
        """
        Step create the Podvm.
        """
        raise NotImplementedError

    @abstractmethod
    def step_install_peerpods(self, log):
        """
        Step install Peer Pods.
        """
        raise NotImplementedError

    def check_commands(self, log):
        """
        Check the required commands are installed in the local system.
        """
        for cmd in self.get_required_commands():
            if which(cmd) is None:
                log.error("ERROR: command '%s' not found", cmd)
                sys.exit(1)

    def check_envvars(self, log):
        """
        Check the required environment variables are exported.
        """
        for envvar in self.get_required_envvars():
            if os.getenv(envvar) is None:
                log.error("ERROR: environment variable '%s' is not exported", envvar)
                sys.exit(1)

    def run(self):
        """
        Run the pipeline on local host.
        """
        ret = 0
        log = logging.getLogger(__name__)

        self.check_commands(log)
        self.check_envvars(log)

        print('\033[95mPIPELINE::SETUP\033[0m')
        if self.pipeline_setup() != 0:
            self.pipeline_teardown()
            sys.exit(1)
        print('\033[95mEND PIPELINE::SETUP\033[0m')

        # Run the steps.
        #
        try:
            self.step_vpc_create(log)
            self.step_cluster_create(log)
            self.step_podvm_create(log)
            self.step_install_peerpods(log)
        except StepException as ex:
            ret = 1
            log.error("ERROR: step failed. Cause: %s", ex)

        print('\033[95mPIPELINE::TEARDOWN\033[0m')
        self.pipeline_teardown()
        print('\033[95mEND PIPELINE::TEARDOWN\033[0m')

        return ret

def step(func):
    """
    Decorate the step function so that start and end messages are printed
    Use to mark the step functions in the implementer class, for example:
    ```
    class ProviderPipeline(PipelineABC)

    @step
    def step_vpc_create(self, log):
        # Provider VPC create implementation.
        pass
    ```
    """
    def wrapper(self, log):
        print('\033[95mSTEP::start::' + func.__name__ + '\033[0m')
        func(self, log)
        print('\033[95mSTEP::end::' + func.__name__ + '\033[0m')
    return wrapper

class StepException(Exception):
    """
    Exception if something bad goes in the step execution.
    """
