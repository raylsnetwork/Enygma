import io
import json
from pprint import pprint
def read_json_file(file_path):
	with open(file_path, 'r') as stream:
		data_loaded = json.load(stream)
	return data_loaded

def save_json_file(file_path, json_dict):
	json_object = json.dumps(json_dict, indent=4)
	 
	# Writing to sample.json
	with open(file_path, "w") as outfile:
	    outfile.write(json_object)

def extract_abi_from_json(src_path, dest_path):
	src_json = read_json_file(src_path)
	src_json.pop('ast', None)
	src_json.pop('opcodes', None)
	src_json.pop('pcMap', None)
	src_json.pop('coverageMap', None)
	src_json.pop('bytecode', None)
	src_json.pop('deployedBytecode', None)
	src_json.pop('source', None)
	src_json.pop('sourceMap', None)
	src_json.pop('deployedSourceMap', None)
	src_json.pop('offset', None)
	src_json.pop('natspec', None)

	# pprint(src_json)
	save_json_file(dest_path, src_json)	

def save_receipts_to_json(receipts_path, receipts):
	receipts_file = open(receipts_path, "w")
	json.dump(receipts, receipts_file,  default=lambda o: o.__dict__, indent=4)
	receipts_file.close()

def save_demo_data_to_json(demo_data_path, demo_dict):
	demo_file = open(demo_data_path, "w")
	json.dump(demo_dict, demo_file,  default=lambda o: o.__dict__, indent=4)
	demo_file.close()	