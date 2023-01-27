// // Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// // SPDX-License-Identifier: MIT

package metrics

// type globalDimensions struct {
// }

// func (ad *globalDimensions) ApplyRule(input interface{}) (returnKey string, returnVal interface{}) {
// 	im := input.(map[string]interface{})

// 	dimensions := map[string]interface{}{}

// 	if _, ok := im["global_dimensions"]; !ok {
// 		returnKey = ""
// 		returnVal = ""
// 	} else {
// 		for key, val := range im["global_dimensions"].(map[string]interface{}) {
// 			if key != "" && val != "" {
// 				dimensions[key] = val
// 			}
// 		}

// 		returnKey = "outputs"
// 		returnVal = map[string]interface{}{
// 			"global_dimensions": dimensions,
// 		}
// 	}
// 	return
// }

// func init() {
// 	gd := new(globalDimensions)
// 	RegisterRule("global_dimensions", gd)
// }
