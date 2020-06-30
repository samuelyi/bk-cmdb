/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.,
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the ",License",); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an ",AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package extensions

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"configcenter/src/ac/meta"
	"configcenter/src/common"
	"configcenter/src/common/blog"
	"configcenter/src/common/condition"
	"configcenter/src/common/metadata"
	"configcenter/src/common/util"
)

/*
 * plat represent cloud plat here
 */

func (am *AuthManager) collectPlatByIDs(ctx context.Context, header http.Header, platIDs ...int64) ([]PlatSimplify, error) {
	rid := util.ExtractRequestIDFromContext(ctx)

	// unique ids so that we can be aware of invalid id if query result length not equal ids's length
	platIDs = util.IntArrayUnique(platIDs)

	cond := metadata.QueryCondition{
		Condition: condition.CreateCondition().Field(common.BKSubAreaField).In(platIDs).ToMapStr(),
	}
	result, err := am.clientSet.CoreService().Instance().ReadInstance(ctx, header, common.BKInnerObjIDPlat, &cond)
	if err != nil {
		blog.V(3).Infof("get plats by id failed, err: %+v, rid: %s", err, rid)
		return nil, fmt.Errorf("get plats by id failed, err: %+v", err)
	}
	plats := make([]PlatSimplify, 0)
	for _, cls := range result.Data.Info {
		plat := PlatSimplify{}
		_, err = plat.Parse(cls)
		if err != nil {
			return nil, fmt.Errorf("get plat by id failed, err: %+v", err)
		}
		plats = append(plats, plat)
	}
	return plats, nil
}

// be careful: plat is registered as a common instance in iam
func (am *AuthManager) MakeResourcesByPlat(header http.Header, action meta.Action, plats ...PlatSimplify) ([]meta.ResourceAttribute, error) {
	ctx := util.NewContextFromHTTPHeader(header)
	rid := util.GetHTTPCCRequestID(header)

	platModels, err := am.collectObjectsByObjectIDs(ctx, header, 0, common.BKInnerObjIDPlat)
	if err != nil {
		blog.Errorf("get plat model failed, err: %+v, rid: %s", err, rid)
		return nil, fmt.Errorf("get plat model failed, err: %+v", err)
	}
	if len(platModels) == 0 {
		blog.Errorf("get plat model failed, not found, rid: %s", rid)
		return nil, fmt.Errorf("get plat model failed, not found")
	}
	platModel := platModels[0]

	resources := make([]meta.ResourceAttribute, 0)
	for _, plat := range plats {
		resource := meta.ResourceAttribute{
			Basic: meta.Basic{
				Action:     action,
				Type:       meta.Plat,
				Name:       plat.BKCloudNameField,
				InstanceID: plat.BKCloudIDField,
			},
			SupplierAccount: util.GetOwnerID(header),
			Layers: []meta.Item{
				{
					Type:       meta.Model,
					Name:       platModel.ObjectName,
					InstanceID: platModel.ID,
				},
			},
		}

		resources = append(resources, resource)
	}
	return resources, nil
}

func (am *AuthManager) AuthorizeByPlat(ctx context.Context, header http.Header, action meta.Action, plats ...PlatSimplify) error {
	if !am.Enabled() {
		return nil
	}

	rid := util.GetHTTPCCRequestID(header)

	// make auth resources
	resources, err := am.MakeResourcesByPlat(header, action, plats...)
	if err != nil {
		blog.Errorf("AuthorizeByPlat failed, MakeResourcesByPlat failed, err: %+v, rid: %s", err, rid)
		return fmt.Errorf("MakeResourcesByPlat failed, err: %s", err.Error())
	}

	return am.authorize(ctx, header, 0, resources...)
}

func (am *AuthManager) AuthorizeByPlatIDs(ctx context.Context, header http.Header, action meta.Action, platIDs ...int64) error {
	if !am.Enabled() {
		return nil
	}

	plats, err := am.collectPlatByIDs(ctx, header, platIDs...)
	if err != nil {
		return fmt.Errorf("get plat by id failed, err: %+d", err)
	}
	return am.AuthorizeByPlat(ctx, header, action, plats...)
}

func (am *AuthManager) ListAuthorizedPlatIDs(ctx context.Context, header http.Header, username string) ([]int64, error) {
	input := meta.ListAuthorizedResourcesParam{
		UserName:     username,
		BizID:        0,
		ResourceType: meta.Plat,
		Action:       meta.FindMany,
	}
	authorizedResources, err := am.clientSet.AuthServer().ListAuthorizedResources(ctx, header, input)
	if err != nil {
		return nil, err
	}

	authorizedPlatIDs := make([]int64, 0)
	for _, resourceID := range authorizedResources {
		// compatible for previous usage
		if strings.HasPrefix(resourceID, "plat:") {
			parts := strings.Split(resourceID, ":")
			if len(parts) < 2 {
				return nil, fmt.Errorf("parse platID from iamResource failed,  iamResourceID: %s, format error", resourceID)
			}
			platID, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse platID from iamResource failed, iamResourceID: %s, err: %+v", resourceID, err)
			}
			authorizedPlatIDs = append(authorizedPlatIDs, platID)
		} else {
			platID, err := strconv.ParseInt(resourceID, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse platID from iamResource failed, iamResourceID: %s, err: %+v", resourceID, err)
			}
			authorizedPlatIDs = append(authorizedPlatIDs, platID)
		}
	}
	return authorizedPlatIDs, nil
}
