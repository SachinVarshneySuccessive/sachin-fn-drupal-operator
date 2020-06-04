#!/usr/bin/env python3
"""
Default setter for opupdate script

To use this script from a different repo, simply copy this file to another repo
and change the values to reflect it's new location.

"""
import os
import opupdate

os.environ['OPERATOR_NAME'] = 'fn-drupal-operator'
os.environ['OPERATOR_REPO'] = 'fn-drupal-operator'
opupdate.main()
