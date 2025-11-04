import yaml
import io
from pprint import pprint

def read_from_file(file_path):
	with open(file_path, 'r') as stream:
		data_loaded = yaml.safe_load(stream)
	return data_loaded

def load_configuration(configuration_path):
	# we can add checks here later
	conf = read_from_file(configuration_path)
	# print("Loaded configuration: ")
	# pprint(conf)
	return conf
