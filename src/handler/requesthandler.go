/*
 *     requesthandler.go is part of github.com/unik-k8s/admission-controller.
 *
 *     Copyright 2023 Markus W Mahlberg <07.federkleid-nagelhaut@icloud.com>
 *
 *     Licensed under the Apache License, Version 2.0 (the "License");
 *     you may not use this file except in compliance with the License.
 *     You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 *     Unless required by applicable law or agreed to in writing, software
 *     distributed under the License is distributed on an "AS IS" BASIS,
 *     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *     See the License for the specific language governing permissions and
 *     limitations under the License.
 *
 */

package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/mwmahlberg/unik-admission-controller/validator"
)

func AdmissionReviewRequesthandler(validator validator.ValidationHandlerV1) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch {
		case r.Body == nil:
			http.Error(w, "no body", http.StatusBadRequest)
			return
		case r.Header.Get("Content-Type") != "application/json":
			http.Error(w, "wrong content type", http.StatusBadRequest)
		}

		content, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body: "+err.Error(), http.StatusBadRequest)
			return
		}

		reviewed := validator.ValidateBytes(content)

		w.Header().Set("Content-Type", "application/json")
		response, err := json.Marshal(reviewed)
		if err != nil {
			http.Error(w, "failed to marshal response: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(response)

	})
}
